package main

import (
	"log"
	"os"

	"github.com/gin-gonic/gin"

	"github.com/gothinkster/golang-gin-realworld-example-app/articles"
	"github.com/gothinkster/golang-gin-realworld-example-app/common"
	"github.com/gothinkster/golang-gin-realworld-example-app/jobs"
	"github.com/gothinkster/golang-gin-realworld-example-app/jobs/worker"
	"github.com/gothinkster/golang-gin-realworld-example-app/users"
	"gorm.io/gorm"
)

func Migrate(db *gorm.DB) {
	users.AutoMigrate()
	db.AutoMigrate(&articles.ArticleModel{})
	db.AutoMigrate(&articles.TagModel{})
	db.AutoMigrate(&articles.FavoriteModel{})
	db.AutoMigrate(&articles.ArticleUserModel{})
	db.AutoMigrate(&articles.CommentModel{})

	// Migrate the Job table
	db.AutoMigrate(&jobs.Job{})
}

func main() {
	db := common.Init()

	// Initialize S3 (LocalStack)
	common.InitS3()

	Migrate(db)

	sqlDB, err := db.DB()
	if err != nil {
		log.Println("failed to get sql.DB:", err)
	} else {
		defer sqlDB.Close()
	}

	// START THE WORKER HERE
	worker.StartWorker()

	r := gin.Default()

	// Disable automatic redirect for trailing slashes
	r.RedirectTrailingSlash = false

	// --- Existing RealWorld API Routes ---
	v1 := r.Group("/api")
	users.UsersRegister(v1.Group("/users"))
	v1.Use(users.AuthMiddleware(false))
	articles.ArticlesAnonymousRegister(v1.Group("/articles"))
	articles.TagsAnonymousRegister(v1.Group("/tags"))
	users.ProfileRetrieveRegister(v1.Group("/profiles"))

	v1.Use(users.AuthMiddleware(true))
	users.UserRegister(v1.Group("/user"))
	users.ProfileRegister(v1.Group("/profiles"))

	articles.ArticlesRegister(v1.Group("/articles"))

	// --- NEW: Bulk Import/Export Routes ---
	// Using /v1 root to strictly follow the assignment requirements
	v1Root := r.Group("/v1")
	jobs.JobsRegister(v1Root)

	// --- Health Check / Ping ---
	testAuth := r.Group("/api/ping")
	testAuth.GET("/", func(c *gin.Context) {
		c.JSON(200, gin.H{
			"message": "pong",
		})
	})

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}
	if err := r.Run(":" + port); err != nil {
		log.Fatal("failed to start server:", err)
	}
}
