package main

import (
  "crypto/rand"
  "crypto/tls"
  "database/sql"
  "encoding/hex"
  "encoding/json"
  "fmt"
  "io"
  "log"
  "math/big"
  "net/http"
  "net/url"
  "os"
  "strings"
  "time"

  "github.com/joho/godotenv"
  "github.com/julienschmidt/httprouter"
  "github.com/dgrijalva/jwt-go"
  "github.com/go-redis/redis"
  "github.com/ttacon/libphonenumber"
  _ "github.com/lib/pq"
)

var listen string
var postgres string
var redisHost string
var secret []byte
var ttl time.Duration
var messagingSID string

var dummyToken string
var coreURL string

var twilioSID string
var twilioToken string

var db *sql.DB
var redisClient *redis.Client

func main() {
  // Load .env
  err := godotenv.Load()
  if err != nil {
    log.Fatal("Error loading .env file")
  }
  listen = os.Getenv("LISTEN")
  secret = []byte(os.Getenv("SECRET"))
  postgres = os.Getenv("POSTGRES")
  redisHost = os.Getenv("REDIS")

  ttl, err = time.ParseDuration(os.Getenv("TTL"))
  if err != nil {
    log.Fatal("Error parsing ttl")
  }

  messagingSID = os.Getenv("MESSAGING_SID")
  twilioSID = os.Getenv("TWILIO_SID")
  twilioToken = os.Getenv("TWILIO_TOKEN")

  dummyToken = "{\"userid\":\"dummy\",\"clientid\":\"dummy\"}"
  coreURL = os.Getenv("CORE_URL")

  // Postgres
  log.Printf("connecting to postgres %s", postgres)
  db, err = sql.Open("postgres", postgres)
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

  // Redis
  redisClient = redis.NewClient(&redis.Options{
    Addr: redisHost,
    Password: "",
    DB: 1,
  })

  // Routes
	router := httprouter.New()

  router.POST("/login", Login);
  router.POST("/init", InitRequest)
  router.POST("/verify", VerifyCode)
  router.POST("/register/:code/:nonce", CreateUser)

  // Start server
  log.Printf("starting server on %s", listen)
	log.Fatal(http.ListenAndServe(listen, router))
}

func ParsePhone(phone string) (string, error) {
	num, err := libphonenumber.Parse(phone, "")
	if err != nil {
		return "", err
	}
	return libphonenumber.Format(num, libphonenumber.INTERNATIONAL), nil
}

func RandomHex() (string, error) {
	b := make([]byte, 16)
	_, err := rand.Read(b)
	return hex.EncodeToString(b), err
}

type InitRequestBody struct {
  PhoneNumber string `json:"phone_number"`
}
func InitRequest(w http.ResponseWriter, r *http.Request, p httprouter.Params) {
  // Get request body
  req := InitRequestBody{}
  decoder := json.NewDecoder(r.Body)
	err := decoder.Decode(&req)
	if err != nil {
		http.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest)
		return
	}

  // Make sure phone number is legitimate
  phone, err := ParsePhone(req.PhoneNumber)
  if err != nil {
    http.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest)
		return
  }

  // Generate OTP code
  c, err := rand.Int(rand.Reader, big.NewInt(1000000))
  code := fmt.Sprintf("%06d", c)

  // Generate nonce
  b := make([]byte, 16)
	_, err = rand.Read(b)
	if err != nil {
    http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}
  bytes := hex.EncodeToString(b)

  // Set code-nonce pair in redis first
  redisClient.Set(code + "nonce", bytes, ttl)
  // Set code-phone_number pair
  redisClient.Set(code + "phone", phone, ttl)

  // Send SMS via Twilio
  data := url.Values {}
  data.Set("MessagingServiceSid", messagingSID)
  data.Set("To", phone)
  data.Set("Body", fmt.Sprintf("Your OTP for Beep is %s", code))

  url := fmt.Sprintf("https://api.twilio.com/2010-04-01/Accounts/%s/Messages.json", twilioSID)
  twilioReq, err := http.NewRequest("POST", url, strings.NewReader(data.Encode()))
  if err != nil {
    http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
  }
  twilioReq.SetBasicAuth(twilioSID, twilioToken)
  twilioReq.Header.Add("Content-Type", "application/x-www-form-urlencoded")

  // Twilio uses self-signed certs
  transport := &http.Transport {
    TLSClientConfig: &tls.Config{ InsecureSkipVerify: true },
  }
  client := &http.Client{ Transport: transport }
  resp, err := client.Do(twilioReq)

  if err != nil {
    http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
  }
  if resp.StatusCode < 200 || resp.StatusCode >= 300 {
    http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
  }

  // Return nonce
  w.Write([]byte(bytes))
}

type VerifyRequestBody struct {
  Code string `json:"code"`
  Nonce string `json:"nonce"`
  ClientId string `json:"clientid"`
}
func VerifyCode(w http.ResponseWriter, r *http.Request, p httprouter.Params) {
  // Get request body
  req := VerifyRequestBody{}
  decoder := json.NewDecoder(r.Body)
	err := decoder.Decode(&req)
	if err != nil {
		http.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest)
		return
	}

  // Get nonce
  storedNonce, err := redisClient.Get(req.Code + "nonce").Result()
  if err != nil {
    http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
  }

  // Delete nonce
  _, err = redisClient.Del(req.Code + "nonce").Result()
  if err != nil {
    http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
  }

  // Check nonce
  if req.Nonce != storedNonce {
    http.Error(w, http.StatusText(http.StatusNotFound), http.StatusNotFound)
		return
  }

  // Get stored phone number
  phoneNumber, err := redisClient.Get(req.Code + "phone").Result()
  if err != nil {
    http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
  }

  // Delete stored phone number
  _, err = redisClient.Del(req.Code + "phone").Result()
  if err != nil {
    http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
  }

  // Generate (potential) User ID
  userHex, err := RandomHex()
  if err != nil {
    http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
  }
  userIDPotential := "u-" + userHex

  // Check for existing user
  var userID string
  err = db.QueryRow(`
		INSERT INTO "user" (id, first_name, last_name, phone_number)
			VALUES ($1, '', '', $2)
			ON CONFLICT(phone_number)
			DO UPDATE SET phone_number=EXCLUDED.phone_number
			RETURNING id
	`, userIDPotential, phoneNumber).Scan(&userID)
  if err != nil {
    http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
    log.Println(err)
    return
  }

  // Generate JWT
  token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims {
    "userid": userID,
    "clientid": req.ClientId,
  })

  tokenString, err := token.SignedString(secret)
  if err != nil {
    http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
    return
  }

  w.Write([]byte(tokenString))
}

type LoginData struct {
  ID string `json:"userid"`
  Client string `json:"clientid"`
}

func Login(w http.ResponseWriter, r *http.Request, p httprouter.Params) {
  login := LoginData {}
  decoder := json.NewDecoder(r.Body)
  err := decoder.Decode(&login)
	if err != nil {
		http.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest)
		return
	}

  token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims {
    "userid": login.ID,
    "clientid": login.Client,
  })

  tokenString, err := token.SignedString(secret)
  if err != nil {
    http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
    return
  }

  w.Write([]byte(tokenString))
}

func CreateUser(w http.ResponseWriter, r *http.Request, p httprouter.Params) {
  code := p.ByName("code")
  nonce := p.ByName("nonce")

  // Get nonce
  storedNonce, err := redisClient.Get(code + "nonce").Result()
  if err != nil {
    http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
  }

  // Delete nonce
  _, err = redisClient.Del(code + "nonce").Result()
  if err != nil {
    http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
  }

  // Check nonce
  if nonce != storedNonce {
    http.Error(w, http.StatusText(http.StatusUnauthorized), http.StatusUnauthorized)
		return
  }

  // Delete phone number
  _, err = redisClient.Del(code + "phone").Result()
  if err != nil {
    http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
  }

  proxyReq, err := http.NewRequest(r.Method, coreURL, r.Body)
  if err != nil {
    http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
    return
  }

  proxyReq.Header.Set("X-User-Claim", dummyToken)
  for header, values := range r.Header {
    for _, value := range values {
      proxyReq.Header.Add(header, value)
    }
  }

  client := &http.Client{}
  proxyRes, err := client.Do(proxyReq)
  if err != nil {
    http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
    return
  }

  for header, values := range proxyRes.Header {
    for _, value := range values {
      w.Header().Add(header, value)
    }
  }
  io.Copy(w, proxyRes.Body)
  proxyRes.Body.Close()
}
