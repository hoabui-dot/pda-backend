package cache

import (
	"context"
	"errors"
)

type Invalidator struct {
	Cache *Aside
	Keys  KeyService
}

func (i Invalidator) clear(c context.Context, w, o string) error {
	if o == "" {
		o = "*"
	}
	var result error
	for _, p := range []string{i.Keys.Key("dashboard", w, o), i.Keys.Key("task-summary", w, o) + "*", i.Keys.Key("inventory-search", w) + "*", i.Keys.Key("inventory-balance", w) + "*", i.Keys.Key("shipment") + "*"} {
		if err := i.Cache.DeletePattern(c, p); err != nil {
			i.Cache.metrics.InvalidationFailure.Add(1)
			result = errors.Join(result, err)
			continue
		}
		i.Cache.metrics.InvalidationSuccess.Add(1)
	}
	return result
}
func (i Invalidator) InvalidateTaskViews(c context.Context, w, o string) error {
	return i.clear(c, w, o)
}
func (i Invalidator) InvalidateReceivingViews(c context.Context, w, o string) error {
	return i.clear(c, w, o)
}
func (i Invalidator) InvalidateMovementViews(c context.Context, w, o string) error {
	return i.clear(c, w, o)
}
func (i Invalidator) InvalidateInventory(c context.Context, w, item, location string) error {
	return i.clear(c, w, "")
}
func (i Invalidator) InvalidateShippingViews(c context.Context, w, o string) error {
	return i.clear(c, w, o)
}
