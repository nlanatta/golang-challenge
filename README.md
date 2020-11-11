# Golang-Challenge
Challenge test

# Doc
````go
type TransparentCache struct {
	sync.Mutex
	actualPriceService PriceService
	maxAge             time.Duration
	prices             map[string]float64
	expirationByItem   map[string]time.Time
}
````
TransparentCache is extending from sync.Mutex in order to be able to lock and unlock writing process in the maps
There two maps, one to keep tracking of prices by code, and other to keep tracking of stored date, for expiration purposes

````go
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
````
Get price is firstly looking into the cache for non expired prices by code, and them if is not there look into the the service
We want to look on write step (storing price and expiration time) and them unlock.

````go
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
	err := <- errOutput
	if err != nil {
		close(errOutput)
	}
	return <- output, err
}
````
GetPricesFor is looking in a concurrent way all prices at once, 
for that purpose is creating 3 channels to receive the list of prices, 
each of the prices and an error if there is an error.
Is creating a wait group to wait for all of them and them 
is closing the channels.
