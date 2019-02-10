package main

import (
  "encoding/json"
  "flag"
  "log"
  "net/http"

  "github.com/julienschmidt/httprouter"
  "github.com/dgrijalva/jwt-go"
)

var listen string
var secret []byte

func main() {
  var s string
  // Parse flags
	flag.StringVar(&listen, "listen", ":8080", "host and port to listen on")
  flag.StringVar(&s, "secret", "secret", "JWT secret")
  flag.Parse()

  secret = []byte(s)

  // Routes
	router := httprouter.New()

  router.POST("/login", Login);

  // Start server
  log.Printf("starting server on %s", listen)
	log.Fatal(http.ListenAndServe(listen, router))
}

type LoginData struct {
  ID string `json:"id"`
  Client string `json:"client"`
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
    "id": login.ID,
    "client": login.Client,
  })

  tokenString, err := token.SignedString(secret)
  if err != nil {
    http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
  }

  w.Write([]byte(tokenString))
}
