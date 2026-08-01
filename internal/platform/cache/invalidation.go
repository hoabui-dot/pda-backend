package cache

import "context"

type Invalidator struct {
	Cache *Aside
	Keys  KeyService
}

func (i Invalidator) clear(c context.Context, w, o string) error {
	if o == "" {
		o = "*"
	}
	for _, p := range []string{i.Keys.Key("dashboard", w, o), i.Keys.Key("task-summary", w, o) + "*", i.Keys.Key("inventory-search", w) + "*", i.Keys.Key("inventory-balance", w) + "*", i.Keys.Key("shipment") + "*"} {
		_ = i.Cache.DeletePattern(c, p)
	}
	return nil
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
