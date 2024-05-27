package main

import (
	"context"
	"errors"
	"os/signal"
	"syscall"
	"time"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"

	"github.com/gin-contrib/graceful"
	log "github.com/sirupsen/logrus"
	"github.com/wwi21seb-projekt/alpha-services/src/api-gateway/handler"
	"github.com/wwi21seb-projekt/alpha-services/src/api-gateway/middleware"
	"github.com/wwi21seb-projekt/alpha-services/src/api-gateway/schema"
	"github.com/wwi21seb-projekt/alpha-shared/config"
	pbPost "github.com/wwi21seb-projekt/alpha-shared/proto/post"
	pbUser "github.com/wwi21seb-projekt/alpha-shared/proto/user"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

var contentTypeHeader = "Content-Type"

func main() {
	// Load configuration
	cfg, err := config.LoadConfig()
	if err != nil {
		log.Fatalf("Failed to load configuration: %v", err)
	}

	// Set up a connection to the gRPC post server
	postConn, err := grpc.NewClient(cfg.PostServiceURL, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		log.Fatalf("Failed to connect to gRPC server: %v", err)
	}
	defer postConn.Close()

	// Set up a connection to the gRPC user server
	userConn, err := grpc.NewClient(cfg.UserServiceURL, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		log.Fatalf("Failed to connect to gRPC server: %v", err)
	}
	defer userConn.Close()

	// Create client stubs
	postClient := pbPost.NewPostServiceClient(postConn)
	userClient := pbUser.NewUserServiceClient(userConn)
	subscriptionClient := pbUser.NewSubscriptionServiceClient(userConn)
	authClient := pbUser.NewAuthenticationServiceClient(userConn)

	// Create handler instances
	postHandler := handler.NewPostHandler(postClient)
	userHandler := handler.NewUserHandler(authClient, userClient, subscriptionClient)

	// Expose HTTP endpoint with graceful shutdown
	r, err := graceful.New(gin.New())
	if err != nil {
		log.Fatal(err)
	}

	setupCommonMiddleware(r)
	apiRouter := r.Group("/api")
	setupRoutes(apiRouter, postHandler, userHandler)
	setupAuthRoutes(apiRouter, postHandler, userHandler)

	// Create a context that listens for termination signals
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	log.Info("Starting server...")
	if err = r.RunWithContext(ctx); err != nil && !errors.Is(err, context.Canceled) {
		log.Fatalf("Server error: %v", err)
	}

	log.Info("Server stopped gracefully")
}

func setupCommonMiddleware(r *graceful.Graceful) {
	r.Use(gin.Logger())
	r.Use(gin.Recovery())
	r.Use(cors.New(cors.Config{
		AllowOrigins:  []string{"*"},
		AllowMethods:  []string{"GET", "PATCH", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowHeaders:  []string{"Accept, Authorization", contentTypeHeader},
		ExposeHeaders: []string{"Content-Length", contentTypeHeader, "X-Correlation-ID"},
		MaxAge:        12 * time.Hour,
	}))
}

// setupRoutes sets up the routes for the API Gateway
func setupRoutes(apiRouter *gin.RouterGroup, postHandler handler.PostHdlr, userHandler handler.UserHdlr) {
	// Post routes
	apiRouter.GET("/feed", postHandler.GetFeed)

	// User routes
	apiRouter.POST("/users", middleware.ValidateAndSanitizeStruct(&schema.RegistrationRequest{}), userHandler.RegisterUser)
	apiRouter.POST("/users/login", middleware.ValidateAndSanitizeStruct(&schema.LoginRequest{}), userHandler.LoginUser)
	apiRouter.POST("users/refresh", middleware.ValidateAndSanitizeStruct(&schema.RefreshTokenRequest{}), userHandler.RefreshToken)
	apiRouter.POST("/users/:username/activate", middleware.ValidateAndSanitizeStruct(&schema.ActivationRequest{}), userHandler.ActivateUser)
	apiRouter.DELETE("/users/:username/activate", userHandler.ResendToken)
}

func setupAuthRoutes(authRouter *gin.RouterGroup, postHandler handler.PostHdlr, userHandler handler.UserHdlr) {
	authRouter.Use(middleware.RequireAuthMiddleware())
	authRouter.Use(middleware.SetClaimsMiddleware())

	// Set user routes
	authRouter.GET("/users", userHandler.SearchUsers)
	authRouter.PUT("/users", middleware.ValidateAndSanitizeStruct(&schema.ChangeTrivialInformationRequest{}), userHandler.ChangeTrivialInfo)
	authRouter.PATCH("/users", middleware.ValidateAndSanitizeStruct(&schema.ChangePasswordRequest{}), userHandler.ChangePassword)
	authRouter.GET("/users/:username", userHandler.GetUser)
	authRouter.GET("/users/:username/feed", userHandler.GetUserFeed)
	authRouter.POST("/subscriptions", middleware.ValidateAndSanitizeStruct(&schema.SubscriptionRequest{}), userHandler.CreateSubscription)
	authRouter.DELETE("/subscriptions/:subscriptionId", userHandler.DeleteSubscription)
	authRouter.GET("/subscriptions/:username", userHandler.GetSubscriptions)

	// Set post routes
	authRouter.POST("posts", middleware.ValidateAndSanitizeStruct(&schema.CreatePostRequest{}), postHandler.CreatePost)
	authRouter.GET("/posts", postHandler.QueryPosts)
	authRouter.DELETE("/posts/:postId", postHandler.DeletePost)
	authRouter.POST("/posts/:postId/comments", middleware.ValidateAndSanitizeStruct(&schema.CreateCommentRequest{}), postHandler.CreateComment)
	authRouter.GET("/posts/:postId/comments", postHandler.GetComments)
	authRouter.POST("/posts/:postId/likes", postHandler.CreateLike)
	authRouter.DELETE("/posts/:postId/likes", postHandler.DeleteLike)
}
