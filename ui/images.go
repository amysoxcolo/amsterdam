/*
 * Amsterdam Web Communities System
 * Copyright (c) 2025-2026 Erbosoft Metaverse Design Solutions, All Rights Reserved
 *
 * This Source Code Form is subject to the terms of the Mozilla Public
 * License, v. 2.0. If a copy of the MPL was not distributed with this
 * file, You can obtain one at https://mozilla.org/MPL/2.0/.
 */

// Package ui holds the support for the Amsterdam user interface, wrapping Echo and Jet templates.
package ui

import (
	"bytes"
	"embed"
	"errors"
	"fmt"
	"image"
	"image/gif"
	"image/jpeg"
	"image/png"
	"mime"
	"mime/multipart"
	"net/http"
	"strconv"
	"strings"

	"git.erbosoft.com/amy/amsterdam/database"
	"github.com/disintegration/imaging"
	"github.com/labstack/echo/v4"
)

//go:embed static_images/*
var static_images embed.FS

//go:embed adbanners/*
var ad_banners embed.FS

// Constants for default photo sizes.
const (
	UserPhotoWidth        = 100
	UserPhotoHeight       = 100
	UserPhotoMaxBytes     = 2097152 // 2 Mb
	CommunityLogoWidth    = 110
	CommunityLogoHeight   = 60
	CommunityLogoMaxBytes = 2097152 // 2 Mb
)

/* mimeTypeFromFilenane returns the MIME type of a file, given its filename.
 * Parameters:
 *     filaname - The name of the file to be tested.
 * Returns:
 *     The file's inferred MIME type.
 */
func mimeTypeFromFilename(filename string) string {
	return mime.TypeByExtension(filename[strings.LastIndex(filename, "."):])
}

/* AmServeImage serves an image from internal storage.
 * Parameters:
 *     c - The Echo context for this request.
 * Returns:
 *     Standard Go error return.
 */
func AmServeImage(c echo.Context) error {
	components := strings.SplitAfter(c.Request().URL.Path, "/")
	var err error = nil
	if len(components) == 4 {
		switch components[2] {
		case "builtin/":
			var b []byte
			b, err = static_images.ReadFile(fmt.Sprintf("static_images/%s", components[3]))
			if err == nil {
				return c.Blob(http.StatusOK, mimeTypeFromFilename(components[3]), b)
			}
		case "ads/":
			var b []byte
			b, err = ad_banners.ReadFile(fmt.Sprintf("adbanners/%s", components[3]))
			if err == nil {
				return c.Blob(http.StatusOK, mimeTypeFromFilename(components[3]), b)
			}
		case "store/":
			var id int
			id, err = strconv.Atoi(components[3])
			if err == nil {
				var img *database.ImageStore
				img, err = database.AmLoadImage(c.Request().Context(), int32(id))
				if err == nil {
					return c.Blob(http.StatusOK, img.MimeType, img.Data)
				}
			}
		}
	}
	if err == nil {
		err = fmt.Errorf("image not found: %s", c.Request().URL.Path)
	}
	return c.String(http.StatusNotFound, err.Error())
}

/* AmServeVeniceCompatibleImage serves an image from the image store under a Venice-compatible URI.
 * Parameters:
 *     c - The Echo context for this request.
 * Returns:
 *     Standard Go error return.
 */
func AmServeVeniceCompatibleImage(c echo.Context) error {
	id, err := strconv.Atoi(c.Param("id"))
	if err == nil {
		var img *database.ImageStore
		img, err = database.AmLoadImage(c.Request().Context(), int32(id))
		if err == nil {
			return c.Blob(http.StatusOK, img.MimeType, img.Data)
		}
	}
	if err == nil {
		err = fmt.Errorf("image not found: %s", c.Request().URL.Path)
	}
	return c.String(http.StatusNotFound, err.Error())
}

/* AmProcessUploadedImage takes an image and resizes it to a specified size, returning its data.
 * Parameters:
 *     fileheader - The multipart file header from the uploaded file.
 *     width - New image width in pizels.
 *     height - New image height in pixels.
 *     maxbytes - The maximum size of the user photo.
 * Returns:
 *     Image data as a byte array.
 *     The MIME type of the image data.
 *     Standard Go error status.
 */
func AmProcessUploadedImage(fileheader *multipart.FileHeader, width, height, maxbytes int) ([]byte, string, error) {
	// test size
	if fileheader.Size > int64(maxbytes) {
		return nil, "", errors.New("file is too large; please try again")
	}

	// open the file
	file, err := fileheader.Open()
	if err != nil {
		return nil, "", err
	}
	defer file.Close()

	// load the image from the file
	img, format, err := image.Decode(file)
	if err != nil {
		return nil, "", err
	}

	// resize the image using high-quality Lanczos filter
	resized := imaging.Resize(img, width, height, imaging.Lanczos)

	// re-encode it to the original format, or JPEG if that's not possible
	var buf bytes.Buffer
	var outType string
	switch strings.ToLower(format) {
	case "jpeg", "jpg":
		err = jpeg.Encode(&buf, resized, &jpeg.Options{Quality: 90})
		outType = "image/jpeg"
	case "gif":
		err = gif.Encode(&buf, resized, &gif.Options{NumColors: 256})
		outType = "image/gif"
	case "png":
		err = png.Encode(&buf, resized)
		outType = "image/png"
	default:
		err = jpeg.Encode(&buf, resized, &jpeg.Options{Quality: 90})
		outType = "image/jpeg"
	}
	if err != nil {
		return nil, "", err
	}
	return buf.Bytes(), outType, nil
}
