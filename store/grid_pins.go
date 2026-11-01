package store

/*
regionMetricPins is derived from Catalog (regimes.go) — region-first regime
categories with cross-signal metrics. Broker oracle sides (volume/day and
parts of spread/book) are painted by Training.lightBroker before Emit.
*/
var regionMetricPins = Catalog.Pins()
