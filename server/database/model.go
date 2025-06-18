package database

type Event struct {
	ID      string `db:"id"`
	Name    string `db:"name"`
	Details string `db:"details"`
}

type Member struct {
	ID          string `db:"id"`
	DisplayName string `db:"display_name"`
	Status      string `db:"status"`
	EventID     string `db:"event_id"`
}
