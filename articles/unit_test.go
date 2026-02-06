package articles

import (
	"net/http/httptest"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/gin-gonic/gin"
	"github.com/gothinkster/golang-gin-realworld-example-app/common"
	"github.com/gothinkster/golang-gin-realworld-example-app/users"
	"github.com/stretchr/testify/assert"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

func TestArticleSerializer(t *testing.T) {
	// Create a mock SQL connection
	sqlDB, _, err := sqlmock.New()
	if err != nil {
		t.Fatalf("Failed to open mock sql db, got error: %v", err)
	}
	defer sqlDB.Close()

	// Initialize GORM with the mock connection
	dialector := postgres.New(postgres.Config{
		Conn:       sqlDB,
		DriverName: "postgres",
	})

	mockGormDB, err := gorm.Open(dialector, &gorm.Config{})
	if err != nil {
		t.Fatalf("Failed to open gorm db, got error: %v", err)
	}

	originalDB := common.DB
	common.DB = mockGormDB
	defer func() { common.DB = originalDB }()

	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)

	// Setup Viewer
	viewer := users.UserModel{ID: 100, Username: "viewer"}
	c.Set("my_user_model", viewer)

	// Setup Article Data
	author := users.UserModel{
		ID:       200,
		Username: "author_name",
		Bio:      "author bio",
	}
	articleAuthor := ArticleUserModel{UserModel: author}

	tag := TagModel{Tag: "golang"}

	article := ArticleModel{
		Slug:        "test-slug",
		Title:       "Test Title",
		Description: "Desc",
		Body:        "Body",
		Author:      articleAuthor,
		Tags:        []TagModel{tag},
	}

	serializer := ArticleSerializer{
		C:            c,
		ArticleModel: article,
	}

	response := serializer.ResponseWithPreloaded(true, 99)

	// Assertions
	assert.Equal(t, "test-slug", response.Slug)
	assert.Equal(t, "Test Title", response.Title)
	assert.Equal(t, "author_name", response.Author.Username)

	assert.False(t, response.Author.Following)

	// Check manual injections
	assert.True(t, response.Favorite)
	assert.Equal(t, uint(99), response.FavoritesCount)

	assert.Len(t, response.Tags, 1)
	assert.Equal(t, "golang", response.Tags[0])
}
