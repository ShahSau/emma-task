package users

import (
	"net/http/httptest"
	"os"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

func TestUserSerializer(t *testing.T) {
	// 1. Setup Environment (Avoid JWT errors)
	os.Setenv("JWT_SECRET", "test-secret")

	// 2. Mock Gin Context
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)

	// 3. Setup Dummy Data
	imageURL := "https://example.com/image.jpg"
	user := UserModel{
		ID:       1,
		Username: "testuser",
		Email:    "test@example.com",
		Bio:      "I am a test user",
		Image:    &imageURL, // Image is a pointer in your model
	}

	// 4. Inject "my_user_model" into Context (Critical Step!)
	// Your serializer calls c.MustGet("my_user_model"), so we must set it here.
	c.Set("my_user_model", user)

	// 5. Initialize Serializer
	serializer := UserSerializer{c: c}

	// 6. Execute
	response := serializer.Response()

	// 7. Assertions
	assert.Equal(t, "testuser", response.Username)
	assert.Equal(t, "test@example.com", response.Email)
	assert.Equal(t, "I am a test user", response.Bio)
	assert.Equal(t, imageURL, response.Image)
	assert.NotEmpty(t, response.Token, "Token should be generated")
}
