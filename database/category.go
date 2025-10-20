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
	"errors"
	"slices"
	"strings"
	"sync"
)

// Category is the structure defining a category.
type Category struct {
	CatId         int32  `db:"catid"`
	Parent        int32  `db:"parent"`
	SymLink       int32  `db:"symlink"`
	HideDirectory bool   `db:"hide_dir"`
	HideSearch    bool   `db:"hide_search"`
	Name          string `db:"name"`
}

// allCategories is the list of all categories loaded from the database.
var allCategories []Category

// categoryIdMap maps IDs to categories.
var categoryIdMap map[int32]*Category = make(map[int32]*Category)

// categoryMutex syncs the loading of the categories.
var categoryMutex sync.Mutex

// loadCategories loads the categories list from the database.
func loadCategories() error {
	categoryMutex.Lock()
	defer categoryMutex.Unlock()
	if allCategories == nil {
		rs, err := amdb.Query("SELECT COUNT(*) FROM refcategory")
		if err != nil {
			return err
		}
		if !rs.Next() {
			return errors.New("internal error loading categories")
		}
		var ncats int32
		rs.Scan(&ncats)
		allCategories = make([]Category, 0, ncats)
		err = amdb.Select(&allCategories, "SELECT * FROM refcategory ORDER BY parent, name")
		if err != nil {
			return err
		}
		for i, c := range allCategories {
			categoryIdMap[c.CatId] = &(allCategories[i])
		}
	}
	return nil
}

/* AmGetCategory returns the category for the given name.
 * Parameters:
 *     catid - The ID of the category to get.
 * Returns:
 *     Pointer to the appropriate Category, or nil.
 *     Standard Go error status.
 */
func AmGetCategory(catid int32) (*Category, error) {
	err := loadCategories()
	if err != nil {
		return nil, err
	}
	c := categoryIdMap[catid]
	d := 5
	for c.SymLink != -1 {
		d--
		if d == 0 {
			return nil, errors.New("symlink resolution error")
		}
		c = categoryIdMap[c.SymLink]
	}
	return c, nil
}

/* AmGetCategoryHierarchy returns the category hierarchy for the given ID.
 * Parameters:
 *     catid - The ID of the category to get.
 * Returns:
 *     Array of pointers to the categories in hierarchical order, or nil.
 *     Standard Go error status.
 */
func AmGetCategoryHierarchy(catid int32) ([]*Category, error) {
	err := loadCategories()
	if err != nil {
		return nil, err
	}
	// walk all the way to the "root" (parent = -1)
	p := catid
	ia := make([]*Category, 0, 3)
	for p != -1 {
		c := categoryIdMap[p]
		for c.SymLink != -1 {
			c = categoryIdMap[c.SymLink]
		}
		ia = append(ia, c)
		p = c.Parent
	}
	// reverse the array for return
	rc := make([]*Category, 0, len(ia))
	for i := range ia {
		rc = append(rc, ia[len(ia)-(i+1)])
	}
	return rc, nil
}

/* AmGetSubCategories returns a list of all subcategories of the given category ID.
 * Parameters:
 *     catid - The parent category ID to use.  May be -1 to return all "top level" categories.
 * Returns:
 *     List of subcategories of this category.
 *     Standard Go error status.
 */
func AmGetSubCategories(catid int32) ([]*Category, error) {
	err := loadCategories()
	if err != nil {
		return nil, err
	}
	rc := make([]*Category, 0)
	for i, cat := range allCategories {
		if catid == cat.Parent {
			rc = append(rc, &(allCategories[i]))
		}
	}
	slices.SortFunc(rc, func(a, b *Category) int {
		return strings.Compare(a.Name, b.Name)
	})
	return rc, nil
}
