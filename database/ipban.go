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
	"fmt"
	"math/big"
	"net"
	"sync"
	"time"
)

// low64mask is bigint 0xFFFFFFFFFFFFFFFF, used in splitting large addresses.
var low64mask *big.Int

// knownBans is a cache of known banned addresses.
var knownBans map[string]string

// knownGood is a cache of known good IP addresses.
var knownGood map[string]bool

// banMutex synchronizes access to our cache.
var banMutex sync.Mutex

// init initializes the internals in this file.
func init() {
	a := big.NewInt(1)
	b := big.NewInt(0).Lsh(a, 64)
	low64mask = big.NewInt(0).Sub(b, big.NewInt(1))
	knownBans = make(map[string]string)
	knownGood = make(map[string]bool)
}

/* AmTestIPBan tests an IP address to see if it's on the banned list.
 * Parameters:
 *     ip_address - The IP address to be tested.
 * Returns:
 *     Ban message if the address is banned, or empty string if it isn't.
 *     Standard Go error status.
 */
func AmTestIPBan(ip_address string) (string, error) {
	banMutex.Lock()
	defer banMutex.Unlock()
	rc := knownBans[ip_address]
	if rc != "" {
		return rc, nil
	}
	if knownGood[ip_address] {
		return "", nil
	}
	addr := net.ParseIP(ip_address)
	if addr == nil {
		return "", fmt.Errorf("invalid address %s", ip_address)
	}
	iv := big.NewInt(0)
	iv.SetBytes(addr)
	iv_lo := big.NewInt(0).And(iv, low64mask).Uint64()
	iv_hi := big.NewInt(0).Rsh(iv, 64).Uint64()
	rows, err := amdb.Query(`
		SELECT message FROM ipban WHERE (address_lo & mask_lo) = (? & mask_lo)
			AND (address_hi & mask_hi) = (? & mask_hi) AND (expire IS NULL OR expire >= ?)
			AND enable <> 0 ORDER BY mask_hi DESC, mask_lo DESC`, iv_lo, iv_hi, time.Now())
	if err != nil {
		return "", err
	}
	defer rows.Close()
	if rows.Next() {
		rows.Scan(&rc)
		knownBans[ip_address] = rc
		return rc, nil
	}
	knownGood[ip_address] = true
	return "", nil
}
