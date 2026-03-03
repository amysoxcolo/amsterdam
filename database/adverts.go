/*
 * Amsterdam Web Communities System
 * Copyright (c) 2025-2026 Erbosoft Metaverse Design Solutions, All Rights Reserved
 *
 * This Source Code Form is subject to the terms of the Mozilla Public
 * License, v. 2.0. If a copy of the MPL was not distributed with this
 * file, You can obtain one at https://mozilla.org/MPL/2.0/.
 */
// The database package contains database management and storage logic.
package database

import (
	"context"
	crand "crypto/rand"
	"math/big"
	"sync"

	"git.erbosoft.com/amy/amsterdam/config"
	lru "github.com/hashicorp/golang-lru"
)

// Advert represents an advertising banner.
type Advert struct {
	AdId      int32   `db:"adid"`      // ID of the ad
	ImagePath string  `db:"imagepath"` // path to the ad image
	PathStyle int16   `db:"pathstyle"` // path style
	Caption   *string `db:"caption"`   // caption
	LinkURL   *string `db:"linkurl"`   // link URL
}

// Values for PathStyle.
const (
	AdPathStyleContextRelative int16 = 0 // indicates context-relative image path
)

// adCache is the cache for advertisements.
var adCache *lru.Cache = nil

// adCacheMutex synchronizes access to adCache.
var adCacheMutex sync.Mutex

// setupAdCache sets up the ad cache.
func setupAdCache() {
	var err error
	adCache, err = lru.New(config.GlobalConfig.Tuning.Caches.Ads)
	if err != nil {
		panic(err)
	}
}

// ResolvePath creates an absolute image path based on the stored path data.
func (ad *Advert) ResolvePath() string {
	if ad.PathStyle == AdPathStyleContextRelative {
		return "/" + ad.ImagePath
	}
	return ""
}

// AmGetAd gets an ad by ID.
func AmGetAd(ctx context.Context, adid int32) (*Advert, error) {
	adCacheMutex.Lock()
	defer adCacheMutex.Unlock()
	rc, ok := adCache.Get(adid)
	if ok {
		return rc.(*Advert), nil
	}
	var theAd Advert
	err := amdb.GetContext(ctx, &theAd, "SELECT * FROM adverts WHERE adid = ?", adid)
	if err != nil {
		return nil, err
	}
	adCache.Add(adid, &theAd)
	return &theAd, nil
}

// AmGetRandomAd gets a random ad from the
func AmGetRandomAd(ctx context.Context) (*Advert, error) {
	var num int
	err := amdb.GetContext(ctx, &num, "SELECT COUNT(*) FROM adverts")
	if err != nil {
		return nil, err
	}
	v1, err := crand.Int(crand.Reader, big.NewInt(int64(num)))
	if err != nil {
		return nil, err
	}
	return AmGetAd(ctx, int32(v1.Int64())+1)
}
