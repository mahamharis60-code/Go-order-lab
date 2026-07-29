package response

import "github.com/gin-gonic/gin"

func OK(c *gin.Context, data any) {
	c.JSON(200, gin.H{"code": 0, "message": "ok", "data": data})
}

func Created(c *gin.Context, data any) {
	c.JSON(201, gin.H{"code": 0, "message": "created", "data": data})
}

func Accepted(c *gin.Context, data any) {
	c.JSON(202, gin.H{"code": 0, "message": "accepted", "data": data})
}

func Error(c *gin.Context, status int, message string) {
	c.JSON(status, gin.H{"code": status, "message": message})
}
