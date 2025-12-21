/*
 * Amsterdam Web Communities System
 * Copyright (c) 2025 Erbosoft Metaverse Design Solutions, All Rights Reserved
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
	"path/filepath"
	"strconv"
	"strings"

	"git.erbosoft.com/amy/amsterdam/database"
	"github.com/disintegration/imaging"
)

//go:embed static_images/*
var static_images embed.FS

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
 *     ctxt - The AmContext for the request.
 * Returns:
 *     Type of content to be rendered
 *     Content to be rendered
 *     Standard Go error return
 */
func AmServeImage(ctxt AmContext) (string, any, error) {
	components := strings.SplitAfter(ctxt.URLPath(), "/")
	var err error = nil
	if len(components) == 4 {
		switch components[2] {
		case "builtin/":
			var b []byte
			b, err = static_images.ReadFile(filepath.Join("static_images", components[3]))
			if err == nil {
				ctxt.SetOutputType(mimeTypeFromFilename(components[3]))
				return "bytes", b, nil
			}
		case "store/":
			var id int
			id, err = strconv.Atoi(components[3])
			if err == nil {
				var img *database.ImageStore
				img, err = database.AmLoadImage(ctxt.Ctx(), int32(id))
				if err == nil {
					ctxt.SetOutputType(img.MimeType)
					return "bytes", img.Data, nil
				}
			}
		}
	}
	ctxt.SetRC(http.StatusNotFound)
	if err == nil {
		err = fmt.Errorf("image not found: %s", ctxt.URLPath())
	}
	return ErrorPage(ctxt, err)
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
