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
	"database/sql"
	"errors"
	"fmt"
	"math/big"
	"net"
	"slices"
	"sync"
	"time"

	"git.erbosoft.com/amy/amsterdam/config"
	log "github.com/sirupsen/logrus"
)

// IPBanEntry represents an IP address banned from the system.
type IPBanEntry struct {
	Id          int32      `db:"id"`         // unique ID of the ban structure
	AddressLow  uint64     `db:"address_lo"` // IP address (low bits)
	AddressHigh uint64     `db:"address_hi"` // IP address (high bits)
	MaskLow     uint64     `db:"mask_lo"`    // Address mask (low bits)
	MaskHigh    uint64     `db:"mask_hi"`    // Address mask (high bits)
	Enable      bool       `db:"enable"`     // is this ban enabled?
	Expire      *time.Time `db:"expire"`     // when does the ban expire?
	Message     string     `db:"message"`    // message to display for ban
	BlockByUid  int32      `db:"block_by"`   // who blocked this IP?
	BlockOn     time.Time  `db:"block_on"`   // when was it blocked?
}

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

// sweepentry is a structure used to communicate with the ban sweeper.
type sweepentry struct {
	expire  time.Time // expiration time
	address string    // IP address in question
}

// banSweeper is a goroutine that sweeps the banned IP address cache, looking for entries that have expired
// and kicking them out so the database can be rechecked.
func banSweeper(done chan bool, ended chan bool, resetMe chan bool, input chan *sweepentry) {
	expireTab := make([]*sweepentry, 0) // table of expiring entries
	running := true
	var topEntry *sweepentry = nil   // always points to the top of expireTab
	var checkTimer *time.Timer = nil // timer indicating when topEntry expires
	for running {
		if checkTimer == nil {
			// "timer idle" mode
			select {
			case <-done:
				running = false
			case <-resetMe:
				// this is bupkis, because we're resetting nothing
				topEntry = nil
				checkTimer = nil
			case entry := <-input:
				// got an initial entry, set the expire timer up
				expireTab = append(expireTab, entry)
				topEntry = entry
				checkTimer = time.NewTimer(time.Until(entry.expire))
			}
		} else {
			// "timer active" mode
			select {
			case <-done:
				running = false
			case <-resetMe:
				// clear out everything on a reset signal
				checkTimer.Stop()
				expireTab = make([]*sweepentry, 0)
				topEntry = nil
				checkTimer = nil
			case entry := <-input:
				// Add new entry to expiretab. Table is always sorted by expire date.
				expireTab = append(expireTab, entry)
				slices.SortFunc(expireTab, func(a, b *sweepentry) int {
					return a.expire.Compare(b.expire)
				})
				if topEntry != expireTab[0] {
					// we have a new top entry! reset the timer
					topEntry = expireTab[0]
					checkTimer.Reset(time.Until(topEntry.expire))
				}
			case <-checkTimer.C:
				// expiry timer fired! kick it out of the known bans hash
				banMutex.Lock()
				delete(knownBans, topEntry.address)
				banMutex.Unlock()
				if len(expireTab) > 1 {
					// got a new top entry, reset the timer
					expireTab = expireTab[1:]
					topEntry = expireTab[0]
					checkTimer.Reset(time.Until(topEntry.expire))
				} else {
					// no more entries, go back to "timer idle" mode
					expireTab = make([]*sweepentry, 0)
					topEntry = nil
					checkTimer = nil
				}
			}
		}
	}
	ended <- true // signal that we're done
}

// banSweeperReset tells the ban sweeper to clear itself.
var banSweeperReset chan bool

// banSweeperInput is the input channel where we feed new entries to the ban sweeper.
var banSweeperInput chan *sweepentry

// setupIPBanSweep sets up the IP ban sweeper routine, and returns a function that tears it down.
func setupIPBanSweep() func() {
	banSweeperReset = make(chan bool)
	banSweeperInput = make(chan *sweepentry, config.GlobalConfig.Tuning.Queues.IPBans)
	done := make(chan bool)
	ended := make(chan bool)
	go banSweeper(done, ended, banSweeperReset, banSweeperInput)
	return func() {
		done <- true
		<-ended
	}
}

// nukeIPBanCache completely clears the IP ban cache.
func nukeIPBanCache(bad, good bool) {
	banMutex.Lock()
	defer banMutex.Unlock()
	if bad {
		banSweeperReset <- true // send the reset signal to the sweeper
		for k := range knownBans {
			delete(knownBans, k)
		}
	}
	if good {
		for k := range knownGood {
			delete(knownGood, k)
		}
	}
}

// IsV4 returns true if the entry's address is an IPv4 address.
func (ipb *IPBanEntry) IsV4() bool {
	ip := AmCombineIP(ipb.AddressLow, ipb.AddressHigh)
	return ip.To4() != nil
}

// SetEnable sets the enable flag on an IP ban entry.
func (ipb *IPBanEntry) SetEnable(ctx context.Context, flag bool) error {
	if flag == ipb.Enable {
		return nil
	}
	_, err := amdb.ExecContext(ctx, "UPDATE ipban SET enable = ? WHERE id = ?", flag, ipb.Id)
	if err == nil {
		nukeIPBanCache(ipb.Enable, !ipb.Enable)
		ipb.Enable = flag
	}
	return err
}

// Delete deletes an IP ban entry.
func (ipb *IPBanEntry) Delete(ctx context.Context) error {
	_, err := amdb.ExecContext(ctx, "DELETE FROM ipban WHERE id = ?", ipb.Id)
	if err == nil {
		nukeIPBanCache(true, false) // can only affect "ban" cache entries, not "good" ones
	}
	return err
}

// AmCombineIP converts an IP address, in terms of low and high 64-bit values, to a net.IP value.
func AmCombineIP(low, high uint64) net.IP {
	t := big.NewInt(0).Lsh(new(big.Int).SetUint64(high), 64)
	addr := big.NewInt(0).Or(t, new(big.Int).SetUint64(low))
	return net.IP(addr.FillBytes(make([]byte, 16)))
}

// AmIPToString converts an IP address, in terms of low and high 64-bit values, to a string.
func AmIPToString(low, high uint64, asV4 bool) string {
	ip := AmCombineIP(low, high)
	if asV4 {
		ip = ip[12:16]
	}
	return ip.String()
}

// AmBreakUpIP breaks up the IP address into low and high uint64 values.
func AmBreakUpIP(addr net.IP) (uint64, uint64) {
	iv := big.NewInt(0).SetBytes(addr)
	ivLo := big.NewInt(0).And(iv, low64mask).Uint64()
	ivHi := big.NewInt(0).Rsh(iv, 64).Uint64()
	return ivLo, ivHi
}

/* AmTestIPBan tests an IP address to see if it's on the banned list.
 * Parameters:
 *     ctx - Standard Go context parameter.
 *     ip_address - The IP address to be tested.
 * Returns:
 *     Ban message if the address is banned, or empty string if it isn't.
 *     Standard Go error status.
 */
func AmTestIPBan(ctx context.Context, ipAddress string) (string, error) {
	banMutex.Lock()
	defer banMutex.Unlock()
	rc := knownBans[ipAddress]
	if rc != "" {
		return rc, nil
	}
	if knownGood[ipAddress] {
		return "", nil
	}
	addr := net.ParseIP(ipAddress)
	if addr == nil {
		return "", fmt.Errorf("invalid address %s", ipAddress)
	}
	ivLo, ivHi := AmBreakUpIP(addr)
	row := amdb.QueryRowContext(ctx, `SELECT message, expire FROM ipban WHERE (address_lo & mask_lo) = (? & mask_lo)
			AND (address_hi & mask_hi) = (? & mask_hi) AND (expire IS NULL OR expire >= NOW())
			AND enable <> 0 ORDER BY mask_hi DESC, mask_lo DESC`, ivLo, ivHi)
	var expire *time.Time = nil
	err := row.Scan(&rc, &expire)
	switch err {
	case nil:
		knownBans[ipAddress] = rc
		if expire != nil {
			// set up so that this entry gets removed when it expires
			banSweeperInput <- &sweepentry{expire: *expire, address: ipAddress}
		}
		return rc, nil
	case sql.ErrNoRows:
		knownGood[ipAddress] = true
		return "", nil
	}
	return "", err
}

// AmListIPBans gets a listing of IP address bans.
func AmListIPBans(ctx context.Context) ([]IPBanEntry, error) {
	var rc []IPBanEntry
	err := amdb.SelectContext(ctx, &rc, "SELECT * FROM ipban")
	return rc, err
}

// AmGetIPBan returns a single IP address ban structure.
func AmGetIPBan(ctx context.Context, id int32) (*IPBanEntry, error) {
	var ban IPBanEntry
	if err := amdb.GetContext(ctx, &ban, "SELECT * FROM ipban WHERE id = ?", id); err != nil {
		return nil, err
	}
	return &ban, nil
}

// AmAddIPBan adds a new IP address ban.
func AmAddIPBan(ctx context.Context, addr, mask net.IP, expires *time.Time, message string, byUser *User) error {
	log.Debugf("AmAddIPBan: addr = %s (%v), mask = %s(%v)", addr.String(), addr, mask.String(), mask)
	if addr.IsUnspecified() {
		return errors.New("cannot add IP ban with unspecified address")
	}
	if mask.IsUnspecified() {
		return errors.New("cannot add IP ban with unspecified mask")
	}
	newEntry := IPBanEntry{
		Expire:     expires,
		Message:    message,
		BlockByUid: byUser.Uid,
	}
	newEntry.AddressLow, newEntry.AddressHigh = AmBreakUpIP(addr)
	if newEntry.AddressLow == 0 && newEntry.AddressHigh == 0 {
		return errors.New("invalid or incorrectly-parsed address")
	}
	newEntry.MaskLow, newEntry.MaskHigh = AmBreakUpIP(mask)
	if newEntry.MaskLow == 0 && newEntry.MaskHigh == 0 {
		return errors.New("invalid or incorrectly-parsed mask")
	}
	_, err := amdb.NamedExecContext(ctx, `INSERT INTO ipban (address_lo, address_hi, mask_lo, mask_hi, enable, expire, message, block_by, block_on)
		VALUES (:address_lo, :address_hi, :mask_lo, :mask_hi, 1, :expire, :message, :block_by, NOW())`, &newEntry)
	if err == nil {
		nukeIPBanCache(false, true)
	}
	return err
}
