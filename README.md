# backend-login

Beep backend handling login. For now, just a POST endpoint returning a JWT. In the furture, SMS-based perpetual login.

## API (temporary)

```
POST /login
```

### Body

| Name | Type | Description | Required |
| ---- | ---- | ----------- | -------- |
| user | String | User's ID. | ✓ |
| device | String | Device's ID. Must be unique to the device. I suggest something based on MAC address. | ✓ |

### Success (200 OK)

JWT token.

### Errors

| Code | Description |
| ---- | ----------- |
| 400 | Required fields in body were not supplied |
| 500 | Error creating the JWT |
