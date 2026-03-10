package main

import (
	"context"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"hris-backend/internal/config"
	"hris-backend/internal/delivery/http/handler"
	"hris-backend/internal/repository/postgres"
	"hris-backend/internal/repository/redis"
	"hris-backend/internal/usecase"
	"hris-backend/pkg/database"
	"hris-backend/pkg/logger"

	"github.com/gin-gonic/gin"
	"github.com/rs/zerolog/log"

	_ "hris-backend/docs"

	swaggerFiles "github.com/swaggo/files"
	ginSwagger "github.com/swaggo/gin-swagger"
)

// @title HRIS Backend API
// @version 1.0
// @description Ini adalah dokumentasi API untuk aplikasi mobile dan admin HRIS.
// @termsOfService http://swagger.io/terms/

// @contact.name Luji API Support
// @contact.email zlfjrii@gmail.com

// @host localhost:3030
// @BasePath /api/v1

// @securityDefinitions.apikey BearerAuth
// @in header
// @name Authorization
func main() {
	// Basic setup
	config.LoadConfig()
	logger.InitLogger()
	database.InitPostgres()
	database.InitRedis()

	// repositories setup
	seqRepo := postgres.NewEmployeeSequenceRepository(database.DB)
	empRepo := postgres.NewEmployeeRepository(database.DB)
	compRepo := postgres.NewCompanyRepository(database.DB)
	otpRepo := redis.NewOTPRepository(database.RedisClient)

	// usecases setup
	empUsecase := usecase.NewEmployeeUsecase(seqRepo, empRepo, compRepo)
	authUsecase := usecase.NewAuthUsecase(empRepo, otpRepo)

	// Setup GIN
	if os.Getenv("APP_ENV") == "production" {
		gin.SetMode(gin.ReleaseMode)
	}
	router := gin.New()
	router.Use(gin.Recovery())

	// swagger setup
	router.GET("/swagger/*any", ginSwagger.WrapHandler(swaggerFiles.Handler))

	apiV1 := router.Group("/api/v1")
	handler.NewEmployeeHandler(apiV1, empUsecase)
	handler.NewAuthHandler(apiV1, authUsecase)

	// Simple Ping Route
	router.GET("/ping", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"message": "pong", "status": "HRIS API is running"})
	})

	// Graceful Shutdown
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
