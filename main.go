package main

import (
	"crypto/subtle"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/nbari/violetear"
	"github.com/nbari/violetear/middleware"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

func catchAll(w http.ResponseWriter, r *http.Request) {
	w.Write([]byte("I'm catching all\n"))
}

func sleep1(w http.ResponseWriter, r *http.Request) {
	time.Sleep(time.Second)
	w.Write([]byte("sleeping 1 second"))
}

func sleep3(w http.ResponseWriter, r *http.Request) {
	time.Sleep(time.Second * 3)
	w.Write([]byte("sleeping 3 seconds"))
}

func sleep5(w http.ResponseWriter, r *http.Request) {
	time.Sleep(time.Second * 5)
	w.Write([]byte("sleeping 5 second"))
}

func loggerMW(c *prometheus.HistogramVec) middleware.Constructor {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()
			rlog := map[string]string{
				"UA":   r.Header.Get("User-Agent"),
				"Time": time.Now().UTC().Format(time.RFC3339),
				"Host": r.Host,
				"URL":  r.URL.String(),
			}
			if b, err := json.Marshal(rlog); err == nil {
				fmt.Println(string(b))
			}
			next.ServeHTTP(w, r)
			endpoint := violetear.GetRouteName(r)
			c.WithLabelValues(endpoint).Observe(time.Since(start).Seconds())
		})
	}
}

func BasicAuth(next http.Handler, username, password, realm string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		user, pass, ok := r.BasicAuth()
		if !ok || subtle.ConstantTimeCompare([]byte(user), []byte(username)) != 1 || subtle.ConstantTimeCompare([]byte(pass), []byte(password)) != 1 {
			w.Header().Set("WWW-Authenticate", `Basic realm="`+realm+`"`)
			http.Error(w, "Unauthorized.", http.StatusUnauthorized)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func main() {
	counter := prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "API",
			Buckets: []float64{0.5, 1, 3, 5, 7},
		}, []string{"endpoint"})
	prometheus.MustRegister(counter)

	stdChain := middleware.New(loggerMW(counter))
	router := violetear.New()
	router.Handle("/metrics", BasicAuth(promhttp.Handler(), "instrument", "everything", "metrics"))
	router.Handle("*", stdChain.ThenFunc(catchAll)).Name("catch-all")
	router.Handle("1s", stdChain.ThenFunc(sleep1)).Name("1s")
	router.Handle("3s", stdChain.ThenFunc(sleep3)).Name("3s")
	router.Handle("5s", stdChain.ThenFunc(sleep5)).Name("5s")

	srv := &http.Server{
		Addr:           ":8080",
		Handler:        router,
		ReadTimeout:    5 * time.Second,
		WriteTimeout:   7 * time.Second,
		MaxHeaderBytes: 1 << 20,
	}
	log.Fatal(srv.ListenAndServe())
}
