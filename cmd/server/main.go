package main

import (
	"context"
	"errors"
	"log"
	"net/http"
	"os/signal"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
	"go-order-lab/internal/config"
	"go-order-lab/internal/database"
	"go-order-lab/internal/handler"
	appmetrics "go-order-lab/internal/metrics"
	"go-order-lab/internal/middleware"
	"go-order-lab/internal/model"
	"go-order-lab/internal/service"
)

func main() {
	cfg := config.Load()
	appCtx, stopSignal := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stopSignal()

	workerCtx, stopWorkers := context.WithCancel(context.Background())
	defer stopWorkers()

	db, err := database.Open(cfg)
	if err != nil {
		log.Fatal(err)
	}

	var redisStock *service.RedisStockStore
	if cfg.RedisEnabled {
		redisStock, err = service.NewRedisStockStore(cfg.RedisAddr, cfg.RedisPassword, cfg.RedisDB)
		if err != nil {
			log.Fatalf("redis enabled but unavailable: %v", err)
		}
		defer redisStock.Close()
		log.Printf("redis stock guard enabled at %s", cfg.RedisAddr)
	}

	var rabbitQueue *service.RabbitQueue
	if cfg.RabbitEnabled {
		rabbitQueue, err = service.NewRabbitQueue(cfg.RabbitURL, cfg.RabbitQueue)
		if err != nil {
			log.Fatalf("rabbitmq enabled but unavailable: %v", err)
		}
		defer rabbitQueue.Close()
		log.Printf("rabbitmq order queue enabled: %s", cfg.RabbitQueue)
	}

	authService := service.NewAuthService(db, cfg.JWTSecret)
	if cfg.AdminUsername != "" {
		if _, err := authService.EnsureAdmin(cfg.AdminUsername, cfg.AdminPassword); err != nil {
			log.Fatalf("ensure admin user: %v", err)
		}
		log.Printf("admin account ready: %s", cfg.AdminUsername)
	}
	catalogService := service.NewCatalogService(db)
	if redisStock != nil {
		catalogService.SetRedisStockStore(redisStock)
	}
	addressService := service.NewAddressService(db)
	cartService := service.NewCartService(db)
	couponService := service.NewCouponService(db)
	orderService := service.NewOrderService(db, redisStock, rabbitQueue)
	adminService := service.NewAdminService(db)
	orderService.SetRabbitMaxRetries(cfg.RabbitMaxRetries)
	orderService.StartWorkersContext(workerCtx, cfg.WorkerCount)
	orderService.StartCompensationWorkerContext(workerCtx, service.CompensationWorkerConfig{
		Enabled:              cfg.CompensationEnabled,
		IntervalSeconds:      cfg.CompensationIntervalSeconds,
		QueuedTimeoutSeconds: cfg.CompensationQueuedTimeoutSeconds,
		PayTimeoutSeconds:    cfg.CompensationPayTimeoutSeconds,
	})

	authHandler := handler.NewAuthHandler(authService)
	catalogHandler := handler.NewCatalogHandler(catalogService)
	addressHandler := handler.NewAddressHandler(addressService)
	cartHandler := handler.NewCartHandler(cartService)
	couponHandler := handler.NewCouponHandler(couponService)
	orderHandler := handler.NewOrderHandler(orderService)
	adminHandler := handler.NewAdminHandler(adminService)

	router := gin.New()
	router.Use(
		gin.Recovery(),
		middleware.TraceID(),
		middleware.Metrics(),
		middleware.AccessLog(),
		middleware.RequestTimeout(time.Duration(cfg.RequestTimeoutSeconds)*time.Second),
	)
	router.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})
	router.GET("/metrics", gin.WrapH(appmetrics.Handler()))

	api := router.Group("/api")
	api.POST("/auth/register", authHandler.Register)
	api.POST("/auth/login", authHandler.Login)
	api.GET("/products", catalogHandler.ListProducts)
	api.GET("/activities", catalogHandler.ListActivities)
	api.GET("/coupons", couponHandler.ListCoupons)
	api.POST("/payments/callback", orderHandler.PaymentCallback)

	protected := api.Group("")
	protected.Use(middleware.Auth(cfg.JWTSecret))
	protected.POST("/addresses", addressHandler.CreateAddress)
	protected.GET("/addresses", addressHandler.ListAddresses)
	protected.POST("/addresses/:id/default", addressHandler.SetDefault)
	protected.POST("/cart/items", cartHandler.AddItem)
	protected.GET("/cart/items", cartHandler.ListItems)
	protected.PATCH("/cart/items/:id", cartHandler.UpdateItem)
	protected.DELETE("/cart/items/:id", cartHandler.DeleteItem)
	protected.POST("/coupons/:id/claim", couponHandler.ClaimCoupon)
	protected.GET("/my-coupons", couponHandler.MyCoupons)
	if cfg.RateLimitEnabled {
		orderLimiter := middleware.NewTokenBucketLimiter(middleware.TokenBucketConfig{
			RPS:   cfg.RateLimitRPS,
			Burst: cfg.RateLimitBurst,
		})
		protected.POST("/orders", orderLimiter.Middleware(), orderHandler.CreateOrder)
	} else {
		protected.POST("/orders", orderHandler.CreateOrder)
	}
	protected.POST("/orders/checkout", orderHandler.CheckoutCart)
	protected.GET("/orders", orderHandler.ListOrders)
	protected.GET("/orders/:order_no", orderHandler.GetOrder)
	protected.POST("/orders/:order_no/cancel", orderHandler.CancelOrder)

	adminOnly := protected.Group("")
	adminOnly.Use(middleware.RequireRole(model.UserRoleAdmin))
	adminOnly.POST("/products", catalogHandler.CreateProduct)
	adminOnly.POST("/activities", catalogHandler.CreateActivity)
	adminOnly.PATCH("/activities/:id/status", catalogHandler.UpdateActivityStatus)
	adminOnly.POST("/coupons", couponHandler.CreateCoupon)
	adminOnly.POST("/orders/expire", orderHandler.ExpireOrders)
	adminOnly.POST("/ops/compensate", orderHandler.Compensate)
	adminOnly.POST("/ops/stock/reconcile", orderHandler.ReconcileStock)
	adminOnly.GET("/admin/overview", adminHandler.Overview)
	adminOnly.GET("/admin/orders", adminHandler.ListOrders)
	adminOnly.GET("/order-logs", orderHandler.ListLogs)

	server := &http.Server{
		Addr:    cfg.Addr,
		Handler: router,
	}
	serverErr := make(chan error, 1)
	go func() {
		log.Printf("go order lab listening on http://127.0.0.1%s", cfg.Addr)
		if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			serverErr <- err
		}
	}()

	select {
	case err := <-serverErr:
		log.Fatal(err)
	case <-appCtx.Done():
		log.Printf("shutdown signal received: %v", appCtx.Err())
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), time.Duration(cfg.ShutdownTimeoutSeconds)*time.Second)
	defer cancel()
	if err := server.Shutdown(shutdownCtx); err != nil {
		log.Printf("http server shutdown error: %v", err)
	}

	stopWorkers()
	workersDone := make(chan struct{})
	go func() {
		orderService.Wait()
		close(workersDone)
	}()
	select {
	case <-workersDone:
		log.Println("background workers stopped")
	case <-shutdownCtx.Done():
		log.Printf("background workers stop timeout: %v", shutdownCtx.Err())
	}

	if sqlDB, err := db.DB(); err == nil {
		if err := sqlDB.Close(); err != nil {
			log.Printf("database close error: %v", err)
		}
	}
	log.Println("server stopped")
}
