package target

import "notifier/cache"

type DataColumnHeader struct {
	Name  string `json:"name"`
	Width int    `json:"width"`
}
type DetailData struct {
	ColNames []DataColumnHeader `json:"colNames"`
	Data     [][]string         `json:"data"`
}

func (d *DetailData) GetDocNo(ttl int64) []string {
	data := []string{}

	cache := cache.GetCache()
	d1 := make(map[string]bool)
	for _, v := range d.Data {
		docno := v[0]
		if _, ok := d1[docno]; !ok {
			d1[docno] = true
			if cache.IsExpired(docno, ttl) {
				data = append(data, docno)
			}
		}
	}

	return data
}
