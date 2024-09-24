package database

import (
	"database/sql"
	"errors"
	"fmt"

	_ "github.com/lib/pq"
)

type Activity struct {
	Activity_id   int
	Spots         int
	Activity_type string
	Room          string
	Speaker       string
	Topic         string
	Description   string
	Time          string
	Day           int
	Timestamp     int64
	Image         string
}

func GetAllActivities() (activities []Activity, err error) {
	query := `
  SELECT *
  FROM activities
  `

	rows, err := DB.Query(query)
	if err != nil {
		return nil, fmt.Errorf("could not retrieve activities: %v", err)
	}
	defer rows.Close()

	for rows.Next() {
		var a Activity
		err := rows.Scan(
			&a.Activity_id,
			&a.Spots,
			&a.Activity_type,
			&a.Room,
			&a.Speaker,
			&a.Topic,
			&a.Description,
			&a.Time,
			&a.Day,
			&a.Timestamp,
			&a.Image,
		)

		if err != nil {
			return nil, fmt.Errorf("could not scan activity: %v", err)
		}
		activities = append(activities, a)
	}

	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("row iteration error: %v", err)
	}

	return activities, nil
}

func GetActivity(id int) (a Activity, err error) {
	query := `
  SELECT *
  FROM activities
  WHERE activities.id = $1
  `

	err = DB.QueryRow(query, id).Scan(
		&a.Activity_id,
		&a.Spots,
		&a.Activity_type,
		&a.Room,
		&a.Speaker,
		&a.Topic,
		&a.Description,
		&a.Time,
		&a.Day,
		&a.Timestamp,
		&a.Image,
	)

	if err != nil {
		if err == sql.ErrNoRows {
			return a, fmt.Errorf("No activity found with id: %v\n", id)
		}
		return a, fmt.Errorf("could not retrieve activity: %v\n", err)
	}

	return a, nil
}

func CreateActivity(a Activity) (int, error) {
	if a.Activity_type != "MC" && a.Activity_type != "PL" {
		return 0, fmt.Errorf("could not create activity, invalid type, not MC or PL")
	}

	query := `
  INSERT INTO activities
  (spots, activity_type, room, speaker, topic, description, time, day, time_stamp, image)
  VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
  RETURNING id
  `
	var id int
	err := DB.QueryRow(query, a.Spots, a.Activity_type, a.Room, a.Speaker, a.Topic, a.Description, a.Time, a.Day, a.Timestamp, a.Image).Scan(&id)
	if err != nil {
		return 0, fmt.Errorf("could not create activity: %v", err)
	}
	return id, nil
}

func (a Activity) String() string {
	return fmt.Sprintf("id: %v | spots: %v | day: %v | time: %v\nroom: %v | type: %v\nspeaker: %v | topic %v\ndescription: %v | timestamp: %v | Image: %v",
		a.Activity_id,
		a.Spots,
		a.Day,
		a.Time,
		a.Room,
		a.Activity_type,
		a.Speaker,
		a.Topic,
		a.Description,
		a.Timestamp,
		a.Image,
	)
}

func SignupUserForActivity(userUUID string, activityID int) (bool, error) {
	tx, err := DB.Begin()
	if err != nil {
		return false, err
	}
	defer tx.Rollback()

	query := `
  SELECT COUNT(*)
  FROM registrations
  WHERE user_id = $1 AND activity_id = $2
  `

	var existingRegistration int
	err = tx.QueryRow(query, userUUID, activityID).Scan(&existingRegistration)
	if err != nil {
		return false, err
	}
	if existingRegistration > 0 {
		return false, errors.New("user is already signed up for this activity")
	}

	var activityDay, activitySpots int
	err = tx.QueryRow("SELECT day, spots FROM activities WHERE id = $1", activityID).Scan(&activityDay, &activitySpots)
	if err != nil {
		return false, err
	}

	query = `
  SELECT COUNT(*) FROM registrations r
  JOIN activities a ON r.activity_id = a.id
  WHERE r.user_id = $1 AND a.day = $2
  `

	var conflictingActivities int
	err = tx.QueryRow(query, userUUID, activityDay).Scan(&conflictingActivities)
	if err != nil {
		return false, err
	}
	if conflictingActivities > 0 {
		return false, errors.New("Você já tem uma atividade nesse dia")
	}

	if activitySpots <= 0 {
		return false, errors.New("Sem vagas disponiveis")
	}

	query = `
  INSERT INTO registrations (user_id, activity_id) 
  VALUES ($1, $2)
  `
	_, err = tx.Exec(query, userUUID, activityID)
	if err != nil {
		return false, err
	}

	query = `
  UPDATE activities
  SET spots = spots - 1
  WHERE id = $1
  `
	_, err = tx.Exec(query, activityID)
	if err != nil {
		return false, err
	}

	err = tx.Commit()
	if err != nil {
		return false, err
	}

	return true, nil
}

func UnregisterUserFromActivity(uuid string, activityID int) error {
	tx, err := DB.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	query := `
    SELECT EXISTS(
        SELECT 1 FROM registrations
        WHERE user_id = $1 AND activity_id = $2
    ), has_attended
    FROM registrations
    WHERE user_id = $1 AND activity_id = $2
    `
	var registrationExists, hasAttended bool
	err = tx.QueryRow(query, uuid, activityID).Scan(&registrationExists, &hasAttended)
	if err != nil {
		if err == sql.ErrNoRows {
			return errors.New("Usuário não registrado nessa atividade")
		}
		return err
	}

	if !registrationExists {
		return errors.New("Usuário não registrado nessa atividade")
	}

	if hasAttended {
		return errors.New("Proibido sair de uma atividade em que participou")
	}

	deleteQuery := `
    DELETE FROM registrations
    WHERE user_id = $1 AND activity_id = $2
    `
	_, err = tx.Exec(deleteQuery, uuid, activityID)
	if err != nil {
		return err
	}

	updateQuery := `
    UPDATE activities
    SET spots = spots + 1
    WHERE id = $1
    `
	_, err = tx.Exec(updateQuery, activityID)
	if err != nil {
		return err
	}

	err = tx.Commit()
	if err != nil {
		return err
	}

	return nil
}

func GetUserActivities(userUUID string) ([]Activity, error) {
	query := `
  SELECT a.id, a.activity_type, a.room, a.speaker, a.topic, a.description, a.time, a.day, a.spots, a.time_stamp, a.image
  FROM activities a
  JOIN registrations r ON a.id = r.activity_id
  WHERE r.user_id = $1
  ORDER BY a.day, a.time
  `

	rows, err := DB.Query(query, userUUID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var activities []Activity
	for rows.Next() {
		var a Activity
		err := rows.Scan(&a.Activity_id, &a.Activity_type, &a.Room, &a.Speaker, &a.Topic, &a.Description, &a.Time, &a.Day, &a.Spots, &a.Timestamp, &a.Image)
		if err != nil {
			return nil, err
		}
		activities = append(activities, a)
	}

	if err = rows.Err(); err != nil {
		return nil, err
	}

	return activities, nil
}

func MarkUserAttendance(uuid string, activityID int) error {
	tx, err := DB.Begin()
	if err != nil {
		return fmt.Errorf("failed to start transaction: %v", err)
	}
	defer tx.Rollback()

	query := `
  UPDATE registrations
  SET has_attended = TRUE
  WHERE user_id = $1 AND activity_id = $2
  `

	result, err := tx.Exec(query, uuid, activityID)
	if err != nil {
		return fmt.Errorf("failed to update attendance status: %v", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %v", err)
	}
	if rowsAffected == 0 {
		return fmt.Errorf("Usuário não está cadastrado nesta atividade")
	}

	if err = tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit transaction: %v", err)
	}

	return nil
}

func UnmarkUserAttendance(uuid string, activityID int) error {
	tx, err := DB.Begin()
	if err != nil {
		return fmt.Errorf("failed to start transaction: %v", err)
	}
	defer tx.Rollback()

	query := `
  UPDATE registrations
  SET has_attended = FALSE
  WHERE user_id = $1 AND activity_id = $2
  `

	result, err := tx.Exec(query, uuid, activityID)
	if err != nil {
		return fmt.Errorf("failed to update attendance status: %v", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %v", err)
	}
	if rowsAffected == 0 {
		return fmt.Errorf("Usuário não está cadastrado nesta atividade")
	}

	if err = tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit transaction: %v", err)
	}

	return nil
}

func GetUserAttendedActivities(uuid string) ([]Activity, error) {
	query := `
  SELECT a.id, a.spots, a.activity_type, a.room, a.speaker, a.topic, a.description, a.time, a.day, a.time_stamp, a.image
  FROM activities a
  JOIN registrations r ON a.id = r.activity_id
  WHERE r.user_id = $1 AND r.has_attended = TRUE
  `

	rows, err := DB.Query(query, uuid)
	if err != nil {
		return nil, fmt.Errorf("failed to query attended activities: %v", err)
	}
	defer rows.Close()

	var attendedActivities []Activity
	for rows.Next() {
		var activity Activity
		err := rows.Scan(
			&activity.Activity_id,
			&activity.Spots,
			&activity.Activity_type,
			&activity.Room,
			&activity.Speaker,
			&activity.Topic,
			&activity.Description,
			&activity.Time,
			&activity.Day,
			&activity.Timestamp,
			&activity.Image,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan activity: %v", err)
		}
		attendedActivities = append(attendedActivities, activity)
	}

	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating over rows: %v", err)
	}

	return attendedActivities, nil
}
