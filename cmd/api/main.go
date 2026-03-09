package main

import (
	"context"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"hris-backend/internal/config"
	"hris-backend/pkg/database"
	"hris-backend/pkg/logger"

	"github.com/gin-gonic/gin"
	"github.com/rs/zerolog/log"
)

func main() {
	// 1. Setup Basic
	config.LoadConfig()
	logger.InitLogger()
	database.InitPostgres()
	database.InitRedis()

	// 2. Setup GIN
	if os.Getenv("APP_ENV") == "production" {
		gin.SetMode(gin.ReleaseMode)
	}
	router := gin.New()
	router.Use(gin.Recovery())

	// Simple Ping Route
	router.GET("/ping", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"message": "pong", "status": "HRIS API is running"})
	})

	// 3. Graceful Shutdown Setup
	srv := &http.Server{
		Addr:    ":" + os.Getenv("APP_PORT"),
		Handler: router,
	}

	go func() {
		log.Info().Msgf("Server is running on port %s", os.Getenv("APP_PORT"))
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatal().Err(err).Msg("Failed to start server")
		}
	}()

	// Wait for interrupt signal to gracefully shutdown the server
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	log.Info().Msg("Shutting down server...")

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := srv.Shutdown(ctx); err != nil {
		log.Fatal().Err(err).Msg("Server forced to shutdown")
	}

	log.Info().Msg("Server exiting")
}