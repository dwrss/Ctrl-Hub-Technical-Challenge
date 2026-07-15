package main

import (
	"context"
	"errors"
	"flag"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	goredis "github.com/redis/go-redis/v9"

	"equipment-exposure-service/internal/app"
	httptransport "equipment-exposure-service/internal/http"
	"equipment-exposure-service/internal/infra/mongodb"
	redisinfra "equipment-exposure-service/internal/infra/redis"
)

type config struct {
	Port        string
	MongoURI    string
	MongoDB     string
	RedisAddr   string
	RedisStream string
}

func loadConfig() config {
	return config{
		Port:        getEnv("PORT", "8080"),
		MongoURI:    getEnv("MONGO_URI", "mongodb://localhost:27017"),
		MongoDB:     getEnv("MONGO_DB", "havs_exposure"),
		RedisAddr:   getEnv("REDIS_ADDR", "localhost:6379"),
		RedisStream: getEnv("REDIS_STREAM", "exposure.recorded"),
	}
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func main() {
	seedFlag := flag.Bool("seed", false, "seed the database with fixture data on startup")
	flag.Parse()

	seed := *seedFlag || getEnv("SEED_ON_STARTUP", "false") == "true"

	cfg := loadConfig()
	ctx := context.Background()

	mongoClient, err := mongodb.Connect(ctx, cfg.MongoURI)
	if err != nil {
		log.Fatalf("failed to connect to mongo: %v", err)
	}
	defer func() {
		if err := mongoClient.Disconnect(context.Background()); err != nil {
			log.Printf("error disconnecting from mongo: %v", err)
		}
	}()

	db := mongodb.Database(mongoClient, cfg.MongoDB)

	redisClient := goredis.NewClient(&goredis.Options{Addr: cfg.RedisAddr})
	defer func() {
		if err := redisClient.Close(); err != nil {
			log.Printf("error closing redis client: %v", err)
		}
	}()
	if err := redisClient.Ping(ctx).Err(); err != nil {
		log.Fatalf("failed to connect to redis: %v", err)
	}

	if seed {
		log.Println("seeding database")
		if err := mongodb.Seed(ctx, db); err != nil {
			log.Fatalf("failed to seed database: %v", err)
		}
		log.Println("database seeded")
	}

	exposureRepo := mongodb.NewExposureRepository(db)
	equipmentRepo := mongodb.NewEquipmentRepository(db)
	userRepo := mongodb.NewUserRepository(db)
	publisher := redisinfra.NewEventPublisher(redisClient, cfg.RedisStream)

	exposureService := app.NewExposureService(exposureRepo, equipmentRepo, userRepo, publisher)

	exposureHandler := httptransport.NewExposureHandler(exposureService)
	summaryHandler := httptransport.NewExposureSummaryHandler(exposureService)

	router := httptransport.NewRouter(exposureHandler, summaryHandler)
	server := httptransport.NewServer(":"+cfg.Port, router)

	go func() {
		log.Printf("listening on :%s", cfg.Port)
		if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatalf("server error: %v", err)
		}
	}()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)
	<-stop

	log.Println("shutting down")
	if err := httptransport.Shutdown(context.Background(), server, 10*time.Second); err != nil {
		log.Printf("error during shutdown: %v", err)
	}
}
