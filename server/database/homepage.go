package database

type Homepage struct {
	ID         int    `db:"homepage_id"`
	Name       string `db:"homepage_name"`
	CustomHost string `db:"homepage_custom_host"`
}

func (d *Database) GetHomepageByHost(host string) (*Homepage, error) {
	var homepage Homepage
	if err := d.db.Get(&homepage, `
		SELECT *
		FROM homepages
		WHERE homepage_custom_host = $1
	`, host); err != nil {
		return nil, err
	}

	return &homepage, nil
}
