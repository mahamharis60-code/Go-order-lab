# Apifox Demo Guide

Use Apifox as the visual interface for backend API documentation, manual testing, and interview demos.

## Environment

Create an environment:

```text
baseUrl = http://<public-ip>:8090
adminToken =
userToken =
```

## Suggested Folders

1. Health and Metrics
   - `GET {{baseUrl}}/health`
   - `GET {{baseUrl}}/metrics`

2. Auth
   - `POST {{baseUrl}}/api/auth/login`
   - `POST {{baseUrl}}/api/auth/register`

3. User APIs
   - product/activity list
   - coupon claim
   - cart item add/list
   - activity order create/list/detail/cancel
   - cart checkout

4. Admin APIs
   - product create
   - activity create/status update
   - coupon create
   - admin overview
   - admin order list
   - operation logs

5. Ops APIs
   - compensate queued and timeout orders
   - stock reconcile and repair

## Demo Flow

1. Check `/health`.
2. Login as admin and save `adminToken`.
3. Create a product.
4. Create a published activity.
5. Register a normal user and save `userToken`.
6. Create an activity order.
7. Wait briefly, then query the order.
8. Send payment callback.
9. Send duplicate payment callback to show idempotency.
10. Query admin overview and order list.
11. Run stock reconcile or compensation if needed.
