package main

type RMADataColumn struct {
	Name  string `json:"name"`
	Width int    `json:"width"`
}
type RMAData struct {
	ColNames []RMADataColumn `json:"colNames"`
	Data     [][]string      `json:"data"`
}

func (d *RMAData) GetDocNo() []string {
	data := []string{}

	cache := GetCache()
	d1 := make(map[string]bool)
	for _, v := range d.Data {
		docno := v[0]
		if _, ok := d1[docno]; !ok {
			d1[docno] = true
			if cache.IsExpired(docno) {
				data = append(data, docno)
			}
		}
	}

	return data
}
