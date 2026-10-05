package main

import (
	"context"
	"encoding/json"
	"fmt"
	mqtt "github.com/eclipse/paho.mqtt.golang"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
	"log"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"os/signal"
	"regexp"
	"strings"
	"syscall"
	"time"
)

var validID = regexp.MustCompile(`^[a-zA-Z0-9_-]{1,64}$`)

type State struct {
	Position   int       `json:"position"`
	ReceivedAt time.Time `json:"received_at"`
}

func send(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}
func fail(w http.ResponseWriter, status int, msg string) {
	send(w, status, map[string]string{"error": msg})
}
func main() {
	if len(os.Args) != 2 {
		log.Fatal("usage: server gateway|devices|telemetry")
	}
	role := os.Args[1]
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	mux := http.NewServeMux()
	if role == "gateway" {
		for prefix, host := range map[string]string{"/api/devices": "devices", "/api/telemetry/": "telemetry"} {
			u, _ := url.Parse("http://" + host + ":8080")
			p := httputil.NewSingleHostReverseProxy(u)
			mux.Handle(prefix, p)
			if prefix == "/api/devices" {
				mux.Handle(prefix+"/", p)
			}
		}
		mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, r *http.Request) { send(w, 200, map[string]string{"status": "ok"}) })
	} else if role == "devices" || role == "telemetry" {
		db, err := pgxpool.New(ctx, os.Getenv("DATABASE_URL"))
		if err != nil {
			log.Fatal(err)
		}
		defer db.Close()
		if err = db.Ping(ctx); err != nil {
			log.Fatal(err)
		}
		cache := redis.NewClient(&redis.Options{Addr: os.Getenv("REDIS_ADDR")})
		defer cache.Close()
		opts := mqtt.NewClientOptions().AddBroker(os.Getenv("MQTT_URL")).SetClientID("ventilation-" + role).SetAutoReconnect(true)
		if role == "telemetry" {
			opts.SetOnConnectHandler(func(c mqtt.Client) {
				token := c.Subscribe("ventilation/+/state", 1, func(c mqtt.Client, m mqtt.Message) {
					parts := strings.Split(m.Topic(), "/")
					if len(parts) != 3 || !validID.MatchString(parts[1]) {
						return
					}
					var s State
					if json.Unmarshal(m.Payload(), &s) != nil || s.Position < 0 || s.Position > 100 {
						return
					}
					s.ReceivedAt = time.Now().UTC()
					work, cancel := context.WithTimeout(context.Background(), 5*time.Second)
					defer cancel()
					if _, err := db.Exec(work, "INSERT INTO telemetry(device_id,position) VALUES($1,$2)", parts[1], s.Position); err != nil {
						log.Printf("store telemetry: %v", err)
						return
					}
					b, _ := json.Marshal(s)
					if err := cache.Set(work, "state:"+parts[1], b, 2*time.Minute).Err(); err != nil {
						log.Printf("cache: %v", err)
					}
				})
				if token.WaitTimeout(10*time.Second) && token.Error() != nil {
					log.Printf("subscribe: %v", token.Error())
				}
			})
		}
		client := mqtt.NewClient(opts)
		token := client.Connect()
		if !token.WaitTimeout(15 * time.Second) {
			log.Fatal("MQTT connect timeout")
		}
		if token.Error() != nil {
			log.Fatal(token.Error())
		}
		defer client.Disconnect(250)
		mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, r *http.Request) {
			c, cancel := context.WithTimeout(r.Context(), 2*time.Second)
			defer cancel()
			if db.Ping(c) != nil || cache.Ping(c).Err() != nil || !client.IsConnected() {
				fail(w, 503, "dependency unavailable")
				return
			}
			send(w, 200, map[string]string{"status": "ok"})
		})
		if role == "devices" {
			mux.HandleFunc("GET /api/devices", func(w http.ResponseWriter, r *http.Request) {
				rows, err := db.Query(r.Context(), "SELECT id,name FROM devices ORDER BY created_at")
				if err != nil {
					fail(w, 500, "database unavailable")
					return
				}
				defer rows.Close()
				list := []map[string]string{}
				for rows.Next() {
					var id, name string
					if rows.Scan(&id, &name) != nil {
						fail(w, 500, "decode failed")
						return
					}
					list = append(list, map[string]string{"id": id, "name": name})
				}
				if rows.Err() != nil {
					fail(w, 500, "query failed")
					return
				}
				send(w, 200, list)
			})
			mux.HandleFunc("POST /api/devices", func(w http.ResponseWriter, r *http.Request) {
				var d struct {
					ID   string `json:"id"`
					Name string `json:"name"`
				}
				r.Body = http.MaxBytesReader(w, r.Body, 4096)
				if json.NewDecoder(r.Body).Decode(&d) != nil || !validID.MatchString(d.ID) || strings.TrimSpace(d.Name) == "" || len(d.Name) > 128 {
					fail(w, 400, "valid id and name required")
					return
				}
				tag, err := db.Exec(r.Context(), "INSERT INTO devices(id,name) VALUES($1,$2) ON CONFLICT DO NOTHING", d.ID, d.Name)
				if err != nil {
					fail(w, 500, "database unavailable")
					return
				}
				if tag.RowsAffected() == 0 {
					fail(w, 409, "device exists")
					return
				}
				send(w, 201, d)
			})
			mux.HandleFunc("POST /api/devices/{id}/position", func(w http.ResponseWriter, r *http.Request) {
				id := r.PathValue("id")
				var body struct {
					Position *int `json:"position"`
				}
				r.Body = http.MaxBytesReader(w, r.Body, 1024)
				if !validID.MatchString(id) || json.NewDecoder(r.Body).Decode(&body) != nil || body.Position == nil || *body.Position < 0 || *body.Position > 100 {
					fail(w, 400, "position must be 0..100")
					return
				}
				var exists bool
				if err := db.QueryRow(r.Context(), "SELECT EXISTS(SELECT 1 FROM devices WHERE id=$1)", id).Scan(&exists); err != nil {
					fail(w, 500, "database unavailable")
					return
				}
				if !exists {
					fail(w, 404, "device missing")
					return
				}
				payload, _ := json.Marshal(body)
				t := client.Publish("ventilation/"+id+"/command", 1, false, payload)
				if !t.WaitTimeout(5*time.Second) || t.Error() != nil {
					fail(w, 503, "MQTT unavailable")
					return
				}
				send(w, 202, map[string]string{"status": "published"})
			})
		} else {
			mux.HandleFunc("GET /api/telemetry/{id}", func(w http.ResponseWriter, r *http.Request) {
				id := r.PathValue("id")
				if !validID.MatchString(id) {
					fail(w, 400, "invalid id")
					return
				}
				var s State
				if b, err := cache.Get(r.Context(), "state:"+id).Bytes(); err == nil && json.Unmarshal(b, &s) == nil {
					send(w, 200, s)
					return
				}
				if err := db.QueryRow(r.Context(), "SELECT position,received_at FROM telemetry WHERE device_id=$1 ORDER BY received_at DESC LIMIT 1", id).Scan(&s.Position, &s.ReceivedAt); err != nil {
					fail(w, 404, "no telemetry")
					return
				}
				send(w, 200, s)
			})
		}
	} else {
		log.Fatal("unknown service")
	}
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
		if r.Method == "OPTIONS" {
			w.WriteHeader(204)
			return
		}
		mux.ServeHTTP(w, r)
	})
	srv := &http.Server{Addr: ":8080", Handler: handler, ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 10 * time.Second, WriteTimeout: 15 * time.Second, IdleTimeout: 60 * time.Second}
	go func() {
		<-ctx.Done()
		c, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		srv.Shutdown(c)
	}()
	fmt.Println("starting", role)
	if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatal(err)
	}
}
