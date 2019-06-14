# backend-login

Beep backend handling login. Call `/init` and then `/verify` in sequence. `/login` is legacy to provide an easy source of tokens for testing, and will be removed someday™.

## Environment variables

Supply environment variables by either exporting them or editing ```.env```.

| ENV | Description | Default |
| ---- | ----------- | ------- |
| LISTEN | Host and port number to listen on | :8080 |
| SECRET | JWT secret | secret |

## API

| Contents |
| -------- |
| Init Auth |
| Verify Code |
| Create Token (temporary) |
| Register User |

---

### Init Auth

```
POST /init
```

Kick off SMS verification process.

#### Body

| Name | Type | Description |
| ---- | ---- | ----------- |
| phone_number | String | Verifying phone number in format `<country code><8 digits>`. |

#### Success (200 OK)

A nonce, to be used for `/verify` to add additional entropy.

#### Errors

| Code | Description |
| ---- | ----------- |
| 400 | Error parsing body/phone_number is not a valid phone number |
| 500 | Error generating nonce/Making request to Twilio SMS |

---

### Verify Code

```
POST /verify
```

Second half of the verification process, verifying the code and returning a JWT. If the user does not exist in the database, a blank one is created.

#### Body

| Name | Type | Description |
| ---- | ---- | ----------- |
| code | String | Verification code received by SMS. |
| nonce | String | Nonce returned by `/init`. |
| clientid | String | ID unique to device, e.g. MAC Address |

#### Success (200 OK)

JWT token.

```json
{
  "userid": "<userid>",
  "clientid": "<clientid>"
}
```

#### Errors

| Code | Description |
| ---- | ----------- |
| 400 | Error parsing body |
| 404 | Code with nonce supplied was not found |
| 500 | Error retrieving record from Redis/querying postgres/creating user ID/generating token |

---

### Create Token (temporary)

```
POST /login
```

Just a simple little endpoint to get a valid token without having to jump through the (expensive) hoops of SMS Authentication.

#### Body

| Name | Type | Description | Required |
| ---- | ---- | ----------- | -------- |
| userid | String | User's ID. | ✓ |
| clientid | String | Device's ID. Must be unique to the device. I suggest something based on MAC address. | ✓ |

#### Success (200 OK)

JWT token.

#### Errors

| Code | Description |
| ---- | ----------- |
| 400 | Required fields in body were not supplied |
| 500 | Error creating the JWT |

---

### Register User

```
POST /register
```

Register a new user. Proxies `core`'s CreateUser endpoint, adding in a dummy token. Admittedly not the most secure implementation ever, but sue me it's 3AM now.

#### Body

| Name | Type | Description | Required |
| ---- | ---- | ----------- | -------- |
| first_name | String | First name of the added user. | ✓ |
| last_name | String | Last name of the added user. | ✓ |
| phone_number | String | Phone number of the added user. Shouldn't be needed but makes life easier. | X |

#### Success Response (200 OK)

Created user object.

```json
{
  "id": "<id>",
  "first_name": "<first_name>",
  "last_name": "<last_name>",
  "phone_number": "<phone_number>"
}
```

#### Errors

| Code | Description |
| ---- | ----------- |
| 400 | Error parsing submitted body, or fields first_name or last_name have a length of 0 |
| 500 | Error occurred inserting entry into database/proxying |
