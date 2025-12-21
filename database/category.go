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
	"errors"
	"slices"
	"strings"
	"sync"

	"git.erbosoft.com/amy/amsterdam/util"
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

// Selectors for operator in category search.
const (
	SearchCatOperPrefix    = 0
	SearchCatOperSubstring = 1
	SearchCatOperRegex     = 2
)

// allCategories is the list of all categories loaded from the database.
var allCategories []Category

// categoryIdMap maps IDs to categories.
var categoryIdMap map[int32]*Category = make(map[int32]*Category)

// categoryMutex syncs the loading of the categories.
var categoryMutex sync.Mutex

// isCatEnabled determines if category features are enabled.
func isCatEnabled(ctx context.Context) (bool, error) {
	g, err := AmGlobals(ctx)
	if err != nil {
		return false, err
	}
	set, err := g.Flags(ctx)
	if err != nil {
		return false, err
	}
	return !set.Get(GlobalFlagNoCategories), nil
}

// loadCategories loads the categories list from the database.
func loadCategories(ctx context.Context) error {
	categoryMutex.Lock()
	defer categoryMutex.Unlock()
	if allCategories == nil {
		rs, err := amdb.QueryContext(ctx, "SELECT COUNT(*) FROM refcategory")
		if err != nil {
			return err
		}
		if !rs.Next() {
			return errors.New("internal error loading categories")
		}
		var ncats int32
		rs.Scan(&ncats)
		allCategories = make([]Category, 0, ncats)
		err = amdb.SelectContext(ctx, &allCategories, "SELECT * FROM refcategory ORDER BY parent, name")
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
 *     ctx - Standard Go context value.
 *     catid - The ID of the category to get.
 * Returns:
 *     Pointer to the appropriate Category, or nil.
 *     Standard Go error status.
 */
func AmGetCategory(ctx context.Context, catid int32) (*Category, error) {
	ok, err := isCatEnabled(ctx)
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, errors.New("category feature not supported")
	}
	err = loadCategories(ctx)
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
 *     ctx - Standard Go context value.
 *     catid - The ID of the category to get.
 * Returns:
 *     Array of pointers to the categories in hierarchical order, or nil.
 *     Standard Go error status.
 */
func AmGetCategoryHierarchy(ctx context.Context, catid int32) ([]*Category, error) {
	ok, err := isCatEnabled(ctx)
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, errors.New("category feature not supported")
	}
	err = loadCategories(ctx)
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
 *     ctx - Standard Go context value.
 *     catid - The parent category ID to use.  May be -1 to return all "top level" categories.
 * Returns:
 *     List of subcategories of this category.
 *     Standard Go error status.
 */
func AmGetSubCategories(ctx context.Context, catid int32) ([]*Category, error) {
	ok, err := isCatEnabled(ctx)
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, errors.New("category feature not supported")
	}
	err = loadCategories(ctx)
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

/* AmSearchCategories searches for categories matching certain criteria.
 * Parameters:
 *     ctx - Standard Go context value.
 *     oper - The operation to perform on the category name:
 *         SearchCatOperPrefix - The category name has the string "term" as a prefix.
 *         SearchCatOperSubstring - The category name contains the string "term".
 *         SearchCatOperRegex - The category name matches the regular expression in "term".
 *     term - The search term, as specified above.
 *     offset - Number of categories to skip at beginning of list.
 *     max - Maximum number of categories to return.
 * Returns:
 *     Array of Category pointers representing the return elements.
 *     The total number of categories matching this query (could be greater than max)
 *	   Standard Go error status.
 */
func AmSearchCategories(ctx context.Context, oper int, term string, offset int, max int, showAll bool, searchAll bool) ([]*Category, int, error) {
	ok, err := isCatEnabled(ctx)
	if err != nil {
		return nil, -1, err
	}
	if !ok {
		return nil, -1, errors.New("category feature not supported")
	}
	var queryString strings.Builder
	queryString.WriteString("name ")
	switch oper {
	case SearchCatOperPrefix:
		queryString.WriteString("LIKE '")
		queryString.WriteString(util.SqlEscape(term, true))
		queryString.WriteString("%'")
	case SearchCatOperSubstring:
		queryString.WriteString("LIKE '%")
		queryString.WriteString(util.SqlEscape(term, true))
		queryString.WriteString("%'")
	case SearchCatOperRegex:
		queryString.WriteString("REGEXP '")
		queryString.WriteString(util.SqlEscape(term, false))
		queryString.WriteString("'")
	default:
		return nil, -1, errors.New("invalid operator to search function")
	}
	if !showAll {
		queryString.WriteString(" AND hide_dir = 0")
	}
	if !searchAll {
		queryString.WriteString(" AND hide_search = 0")
	}
	q := queryString.String()
	rs, err := amdb.QueryContext(ctx, "SELECT COUNT(*) FROM refcategory WHERE "+q)
	if err != nil {
		return nil, -1, err
	}
	if !rs.Next() {
		return nil, -1, errors.New("internal error getting category total")
	}
	var total int
	rs.Scan(&total)
	if total == 0 {
		return make([]*Category, 0), 0, nil
	}
	if offset > 0 {
		rs, err = amdb.QueryContext(ctx, "SELECT catid FROM refcategory WHERE "+q+" ORDER BY parent, name LIMIT ? OFFSET ?", max, offset)
	} else {
		rs, err = amdb.QueryContext(ctx, "SELECT catid FROM refcategory WHERE "+q+" ORDER BY parent, name LIMIT ?", max)
	}
	if err != nil {
		return nil, total, err
	}
	rc := make([]*Category, 0, min(max, 1000))
	for rs.Next() {
		var catid int32
		rs.Scan(&catid)
		c, err := AmGetCategory(ctx, catid)
		if err == nil {
			rc = append(rc, c)
		}
	}
	return rc, total, nil
}
