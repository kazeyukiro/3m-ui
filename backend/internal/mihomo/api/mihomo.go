package api

func (c *Client) Traffic() (*Traffic, error) {
	var result Traffic
	err := c.get("/traffic", &result)
	return &result, err
}

func (c *Client) Connections() (*Connections, error) {
	var result Connections
	err := c.get("/connections", &result)
	return &result, err
}
