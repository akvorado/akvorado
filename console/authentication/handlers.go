// SPDX-FileCopyrightText: 2022 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

package authentication

import (
	"bufio"
	"fmt"
	"hash/fnv"
	"image"
	"image/draw"
	"image/png"
	"io/fs"
	"math/rand/v2"
	"net/http"

	"akvorado/common/embed"

	"github.com/gin-gonic/gin"
)

// UserInfoHandlerFunc returns the information about the currently logged user.
func (c *Component) UserInfoHandlerFunc(gc *gin.Context) {
	info := gc.MustGet("user").(UserInformation)
	gc.JSON(http.StatusOK, info)
}

// UserAvatarHandlerFunc returns an avatar for the currently logger user.
func (c *Component) UserAvatarHandlerFunc(gc *gin.Context) {
	// Hash user login as a source
	info := gc.MustGet("user").(UserInformation)
	hash := fnv.New64()
	hash.Write([]byte(info.Login))
	randSource := rand.New(rand.NewPCG(hash.Sum64(), 0))
	etag := fmt.Sprintf(`"%x"`, hash.Sum64())

	// Do we have a If-None-Match header?
	if header := gc.GetHeader("If-None-Match"); header == etag {
		gc.Header("ETag", etag)
		gc.Status(http.StatusNotModified)
		return
	}

	// Grab list of parts
	avatarParts, err := fs.Sub(embed.Data(), "console/authentication")
	if err != nil {
		c.r.Err(err).Msg("cannot open embedded archive")
		gc.JSON(http.StatusInternalServerError, gin.H{"message": "Cannot build avatar."})
		return
	}
	parts := []string{}
	partList, err := avatarParts.Open("data/avatars/partlist.txt")
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
		p, _ := fs.Glob(avatarParts, fmt.Sprintf("data/avatars/%s_*", part))
		if len(p) == 0 {
			c.r.Error().Msgf("missing part %s", part)
			gc.JSON(http.StatusInternalServerError, gin.H{"message": "Cannot build avatar."})
			return
		}
		parts[idx] = p[randSource.IntN(len(p))]
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
	gc.Header("ETag", etag)
	gc.Status(http.StatusOK)
	png.Encode(gc.Writer, img)
}
