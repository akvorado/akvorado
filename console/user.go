package console

import (
	"bufio"
	"embed"
	"fmt"
	"hash/fnv"
	"image"
	"image/draw"
	"image/png"
	"io/fs"
	"math/rand"
	"net/http"
	"regexp"

	"github.com/gin-gonic/gin"
)

var (
	//go:embed data/avatars
	embeddedAvatarParts embed.FS
	avatarRegexp        = regexp.MustCompile(`^([a-z]+)_([0-9]+)\.png$`)
)

// UserInformation contains information about the current user.
type userInformation struct {
	Login     string `json:"login" header:"X-Akvorado-User-Login" binding:"required"`
	Name      string `json:"name,omitempty" header:"X-Akvorado-User-Name"`
	Email     string `json:"email,omitempty" header:"X-Akvorado-User-Email" binding:"omitempty,email"`
	AvatarURL string `json:"avatar-url" header:"X-Akvorado-User-Avatar" binding:"omitempty,uri"`
	LogoutURL string `json:"logout-url,omitempty" header:"X-Akvorado-User-Logout" binding:"omitempty,uri"`
}

// UserAuthentication is a middleware to fill information about the
// current user. It does not really perform authentication but relies
// on HTTP headers.
func (c *Component) userAuthentication() gin.HandlerFunc {
	return func(gc *gin.Context) {
		var info userInformation
		if err := gc.ShouldBindHeader(&info); err != nil {
			gc.Next()
			return
		}
		if info.AvatarURL == "" {
			info.AvatarURL = "/api/v0/console/user/avatar"
		}
		gc.Set("user", info)
		gc.Next()
	}
}

// requireUserAuthentication requires user to be logged in. It returns
// 401 if not.
func (c *Component) requireUserAuthentication() gin.HandlerFunc {
	return func(gc *gin.Context) {
		_, ok := gc.Get("user")
		if !ok {
			gc.JSON(http.StatusUnauthorized, gin.H{"message": "No user logged in."})
			gc.Abort()
			return
		}
		gc.Next()
	}
}

// userInfoHandlerFunc returns the information about the currently logged user.
func (c *Component) userInfoHandlerFunc(gc *gin.Context) {
	info := gc.MustGet("user").(userInformation)
	gc.JSON(http.StatusOK, info)
}

// userAvatarHandlerFunc returns an avatar for the currently logger user.
func (c *Component) userAvatarHandlerFunc(gc *gin.Context) {
	avatarParts := c.embedOrLiveFS(embeddedAvatarParts, "data/avatars")

	// Hash user login as a source
	info := gc.MustGet("user").(userInformation)
	hash := fnv.New64()
	hash.Write([]byte(info.Login))
	randSource := rand.New(rand.NewSource(int64(hash.Sum64())))

	// Grab list of parts
	parts := []string{}
	partList, err := avatarParts.Open("partlist.txt")
	if err != nil {
		c.r.Err(err).Msg("cannot open partlist.txt")
		gc.JSON(http.StatusInternalServerError, gin.H{"message": "Cannot build avatar."})
		return
	}
	defer partList.Close()
	scanner := bufio.NewScanner(partList)
	for scanner.Scan() {
		parts = append(parts, scanner.Text())
	}

	// Choose an image for each part
	for idx, part := range parts {
		// Choose a file for each part
		p, _ := fs.Glob(avatarParts, fmt.Sprintf("%s_*", part))
		if len(p) == 0 {
			c.r.Error().Msgf("missing part %s", part)
			gc.JSON(http.StatusInternalServerError, gin.H{"message": "Cannot build avatar."})
			return
		}
		parts[idx] = p[randSource.Intn(len(p))]
	}

	// Compose the images
	var img *image.RGBA
	for _, part := range parts {
		filePart, err := avatarParts.Open(part)
		if err != nil {
			c.r.Err(err).Msgf("cannot open part %s", part)
			gc.JSON(http.StatusInternalServerError, gin.H{"message": "Cannot build avatar."})
			return
		}
		imgPart, err := png.Decode(filePart)
		filePart.Close()
		if err != nil {
			c.r.Err(err).Msgf("cannot decode part %s", part)
			gc.JSON(http.StatusInternalServerError, gin.H{"message": "Cannot build avatar."})
			return
		}
		if img == nil {
			img = image.NewRGBA(imgPart.Bounds())
		}
		draw.Draw(img, img.Bounds(), imgPart, imgPart.Bounds().Min, draw.Over)
	}

	// Serve the result
	gc.Header("Content-Type", "image/png")
	gc.Header("Cache-Control", "max-age=86400")
	gc.Header("Vary", "X-Akvorado-User-Login")
	gc.Status(http.StatusOK)
	png.Encode(gc.Writer, img)
}
