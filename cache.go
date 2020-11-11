package main

import (
	"fmt"
	"sync"
	"time"
)

// PriceService is a service that we can use to get prices for the items
// Calls to this service are expensive (they take time)
type PriceService interface {
	GetPriceFor(itemCode string) (float64, error)
}

// TransparentCache is a cache that wraps the actual service
// The cache will remember prices we ask for, so that we don't have to wait on every call
// Cache should only return a price if it is not older than "maxAge", so that we don't get stale prices
type TransparentCache struct {
	sync.Mutex
	actualPriceService PriceService
	maxAge             time.Duration
	prices             map[string]float64
	expirationByItem   map[string]time.Time
}

//Create new Cache
func NewTransparentCache(actualPriceService PriceService, maxAge time.Duration) *TransparentCache {
	return &TransparentCache{
		actualPriceService: actualPriceService,
		maxAge:             maxAge,
		prices:             map[string]float64{},
		expirationByItem:   map[string]time.Time{},
	}
}

// GetPriceFor gets the price for the item, either from the cache or the actual service if it was not cached or too old
func (c *TransparentCache) GetPriceFor(itemCode string) (float64, error) {
	price, ok := c.prices[itemCode]
	if ok {
		if c.expirationByItem[itemCode].Add(c.maxAge).After(time.Now()) {
			return price, nil
		}
	}
	price, err := c.actualPriceService.GetPriceFor(itemCode)
	if err != nil {
		return 0, fmt.Errorf("getting price from service : %v", err.Error())
	}
	c.Lock()
	defer c.Unlock()
	c.prices[itemCode] = price
	c.expirationByItem[itemCode] = time.Now()
	return price, nil
}

// GetPricesFor gets the prices for several items at once, some might be found in the cache, others might not
// If any of the operations returns an error, it should return an error as well
func (c *TransparentCache) GetPricesFor(itemCodes ...string) ([]float64, error) {
	var w sync.WaitGroup
	output := make(chan []float64)
	input := make(chan float64)
	errOutput := make(chan error)
	defer close(output)

	go c.handleResults(input, output, &w)
	for _, itemCode := range itemCodes {
		w.Add(1)
		go c.getConcurrentPrice(input, itemCode, errOutput)
	}
	w.Wait()
	close(input)
	err := <-errOutput
	if err != nil {
		close(errOutput)
	}
	return <-output, err
}

// Handle price input channels and output prices channel
func (c *TransparentCache) handleResults(input chan float64, output chan []float64, wg *sync.WaitGroup) {
	var results []float64
	for result := range input {
		results = append(results, result)
		wg.Done()
	}
	output <- results
}

// Get concurrent price into output channel or through an error into error channel
func (c *TransparentCache) getConcurrentPrice(input chan float64, itemCode string, errOutput chan error) {
	price, err := c.GetPriceFor(itemCode)
	input <- price
	errOutput <- err
}
