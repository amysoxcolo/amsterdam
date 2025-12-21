/*
 * Amsterdam Web Communities System
 * Copyright (c) 2025 Erbosoft Metaverse Design Solutions, All Rights Reserved
 *
 * This Source Code Form is subject to the terms of the Mozilla Public
 * License, v. 2.0. If a copy of the MPL was not distributed with this
 * file, You can obtain one at https://mozilla.org/MPL/2.0/.
 */
// The database package contains database management and storage logic.
package database

import (
	"context"
	"database/sql"
	"fmt"
)

// ImageStore is the structure for the image store table.
type ImageStore struct {
	ImgId    int32  `db:"imgid"`
	TypeCode int16  `db:"typecode"`
	OwnerID  *int32 `db:"ownerid"`
	MimeType string `db:"mimetype"`
	Length   int32  `db:"length"`
	Data     []byte `db:"data"`
}

// Values for the TypeCode field of ImageStore.
const (
	ImageTypeUserPhoto     = int16(1)
	ImageTypeCommunityLogo = int16(2)
)

// Save persists the ImageStore record to the database.
func (img *ImageStore) Save(ctx context.Context) error {
	var err error
	if img.ImgId > 0 {
		_, err = amdb.NamedExecContext(ctx, `UPDATE imagestore SET typecode = :typecode, ownerid = :ownerid, mimetype = :mimetype,
		    length = :length, data = :data WHERE imgid = :imgid`, img)
	} else {
		var rs sql.Result
		rs, err = amdb.NamedExecContext(ctx, `INSERT INTO imagestore (typecode, ownerid, mimetype, length, data)
			VALUES (:typecode, :ownerid, :mimetype, :length, :data)`, img)
		if err == nil {
			var lii int64
			lii, err = rs.LastInsertId()
			if err == nil {
				img.ImgId = int32(lii)
			}
		}
	}
	return err
}

/* AmLoadImage loads an image from the database.
 * Parameters:
 *     ctx - Standard Go context value.
 *     id - The ID of the image to be loaded.
 * Returns:
 *     Pointer to ImageStore, or nil.
 *     Standard Go error status.
 */
func AmLoadImage(ctx context.Context, id int32) (*ImageStore, error) {
	var dbdata []ImageStore
	err := amdb.SelectContext(ctx, &dbdata, "SELECT * FROM imagestore WHERE imgid = ?", id)
	if err != nil {
		return nil, err
	}
	if len(dbdata) == 0 {
		return nil, fmt.Errorf("image ID %d not found", id)
	} else if len(dbdata) > 1 {
		return nil, fmt.Errorf("image ID %d too many images (%d)", id, len(dbdata))
	}
	return &(dbdata[0]), nil
}

/* AmStoreImage stores an image in the database, overwriting one with the same type code and owner if it exists.
 * Parameters:
 *     ctx - Standard Go context value.
 *     typecode - Type code for the image.
 *     owner - Owner Id for the image (UID or community ID)
 *     mimetype - MIME type of the image.
 *     data - Bytes of the actual image.
 * Returns:
 *     Pointer to ImageStore, or nil.
 *     Standard Go error status.
 */
func AmStoreImage(ctx context.Context, typecode int16, owner int32, mimetype string, data []byte) (*ImageStore, error) {
	rs, err := amdb.QueryContext(ctx, "SELECT imgid FROM imagestore WHERE typecode = ? AND ownerid = ?", typecode, owner)
	if err != nil {
		return nil, err
	}
	var img *ImageStore
	if rs.Next() {
		var id int32
		rs.Scan(&id)
		img, err = AmLoadImage(ctx, id)
		if err != nil {
			return nil, err
		}
		img.MimeType = mimetype
		img.Length = int32(len(data))
		img.Data = data
	} else {
		img = &ImageStore{
			ImgId:    -1,
			TypeCode: typecode,
			OwnerID:  &owner,
			MimeType: mimetype,
			Length:   int32(len(data)),
			Data:     data,
		}
	}
	err = img.Save(ctx)
	if err != nil {
		return nil, err
	}
	return img, nil
}

/* AmDeleteImage erases an image from the database.
 * Parameters:
 *     ctx - Standard Go context value.
 *     id - The ID of the image to be deleted.
 * Returns:
 *     Standard Go error status.
 */
func AmDeleteImage(ctx context.Context, id int32) error {
	_, err := amdb.ExecContext(ctx, "DELETE FROM imagestore WHERE imgid = ?", id)
	return err
}
