package tests

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/gothinkster/golang-gin-realworld-example-app/articles"
	"github.com/gothinkster/golang-gin-realworld-example-app/common"
	"github.com/gothinkster/golang-gin-realworld-example-app/users"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func setupRouter() *gin.Engine {
	r := gin.Default()
	r.RedirectTrailingSlash = false

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

	return r
}

func setupDB() {
	var err error
	common.DB, err = gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		panic("failed to connect database")
	}

	err = common.DB.AutoMigrate(
		&users.UserModel{},
		&articles.ArticleModel{},
		&articles.TagModel{},
		&articles.CommentModel{},
		&users.FollowModel{},
		&articles.FavoriteModel{},
		&articles.ArticleUserModel{},
	)
	if err != nil {
		panic("failed to migrate database")
	}
}

func TestMain(m *testing.M) {
	gin.SetMode(gin.TestMode)
	setupDB()
	os.Exit(m.Run())
}

func TestGetArticles_Integration(t *testing.T) {
	router := setupRouter()

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/articles", nil)
	router.ServeHTTP(w, req)

	if w.Code == 404 {
		t.Log("DEBUG: Routes registered:")
		for _, r := range router.Routes() {
			t.Logf("%s %s", r.Method, r.Path)
		}
	}

	require.Equal(t, http.StatusOK, w.Code)

	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)

	assert.Contains(t, response, "articles")
	assert.Contains(t, response, "articlesCount")
	assert.Equal(t, float64(0), response["articlesCount"])
}
