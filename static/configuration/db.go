package configuration

import (
	"fmt"
	"strings"

	"github.com/blocklords/sds/common/blockchain"
	"github.com/blocklords/sds/common/smartcontract_key"
	"github.com/blocklords/sds/common/topic"
	"github.com/blocklords/sds/db"
	"github.com/blocklords/sds/static/smartcontract"
)

// Inserts the configuration into the database
// It doesn't validates the configuration.
// Call conf.Validate() before calling this
func SetInDatabase(db *db.Database, conf *Configuration) error {
	result, err := db.Connection.Exec(`INSERT IGNORE INTO static_configuration (organization, project, network_id, group_name, smartcontract_name, address) VALUES (?, ?, ?, ?, ?, ?) `,
		conf.Topic.Organization, conf.Topic.Project, conf.Topic.NetworkId, conf.Topic.Group, conf.Topic.Smartcontract, conf.Address)
	if err != nil {
		fmt.Println("Failed to insert static configuration")
		return err
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("checking insert result: %w", err)
	}
	if affected != 1 {
		return fmt.Errorf("expected to have 1 affected rows. Got %d", affected)
	}

	return nil

}

// Creates a database query
// against the topic filter
func query_parameters(t *topic.TopicFilter) (string, []string) {
	query := ""
	args := make([]string, 0)

	l := len(t.Organizations)
	if l > 0 {
		query += ` AND c.organization IN (?` + strings.Repeat(",?", l-1) + `)`
		args = append(args, t.Organizations...)
	}

	l = len(t.Projects)
	if l > 0 {
		query += ` AND c.project IN (?` + strings.Repeat(",?", l-1) + `)`
		args = append(args, t.Projects...)
	}

	l = len(t.NetworkIds)
	if l > 0 {
		query += ` AND c.network_id IN (?` + strings.Repeat(",?", l-1) + `)`
		args = append(args, t.NetworkIds...)
	}

	l = len(t.Groups)
	if len(t.Groups) > 0 {
		query += ` AND c.group_name IN (?` + strings.Repeat(",?", l-1) + `)`
		args = append(args, t.Groups...)
	}

	l = len(t.Smartcontracts)
	if len(t.Smartcontracts) > 0 {
		query += ` AND c.smartcontract_name IN (?` + strings.Repeat(",?", l-1) + `)`
		args = append(args, t.Smartcontracts...)
	}

	return query, args
}

// Returns the static smartcontracts by filter_query
// The filter_query is generated by categorizer/configuration from topic_filter
// Returns the list of smartcontracts by topic filter.
// Each smartcontract has the topic filter that matches to this contract
func FilterSmartcontracts(con *db.Database, topic_filter *topic.TopicFilter) ([]smartcontract.Smartcontract, []topic.Topic, error) {
	filter_query, filter_parameters := query_parameters(topic_filter)
	query := `
		SELECT 
			s.network_id, 
			s.address, 
			s.abi_id, 
			s.transaction_id, 
			s.transaction_index, 
			s.block_number, 
			s.block_timestamp, 
			s.deployer,
			c.organization, 
			c.project, 
			c.group_name, 
			c.smartcontract_name
		FROM 
			static_smartcontract AS s, 
			static_configuration AS c 
		WHERE
			s.network_id = c.network_id AND 
			s.address = c.smartcontract_address
	` + filter_query

	args := make([]interface{}, len(filter_parameters))
	for i, param := range filter_parameters {
		args[i] = param
	}

	rows, err := con.Connection.Query(query, args...)
	if err != nil {
		return nil, nil, err
	}
	defer rows.Close()

	var smartcontracts []smartcontract.Smartcontract
	var topics []topic.Topic

	// Loop through rows, using Scan to assign column data to struct fields.
	for rows.Next() {
		var s smartcontract.Smartcontract = smartcontract.Smartcontract{
			SmartcontractKey: smartcontract_key.Key{},
			TransactionKey:   blockchain.TransactionKey{},
			BlockHeader:      blockchain.BlockHeader{},
		}

		var t topic.Topic
		if err := rows.Scan(&s.SmartcontractKey.NetworkId, &s.SmartcontractKey.Address, &s.AbiId, &s.TransactionKey.Id, &s.TransactionKey.Index, &s.BlockHeader.Number, &s.BlockHeader.Timestamp, &s.Deployer,
			&t.Organization, &t.Project, &t.Group, &t.Smartcontract); err != nil {
			return nil, nil, err
		}
		t.NetworkId = s.SmartcontractKey.NetworkId
		smartcontracts = append(smartcontracts, s)
		topics = append(topics, t)
	}
	return smartcontracts, topics, nil
}

func FilterSmartcontractKeys(con *db.Database, topic_filter *topic.TopicFilter) ([]smartcontract_key.Key, []*topic.Topic, error) {
	filter_query, filter_parameters := query_parameters(topic_filter)

	query := `SELECT 
		static_smartcontract.network_id, 
		static_smartcontract.address, 
		c.organization, 
		c.project, 
		c.group_name, 
		c.smartcontract_name
	FROM 
		static_smartcontract, 
		static_configuration AS c
	WHERE
		static_smartcontract.network_id = c.network_id AND 
		static_smartcontract.address = c.smartcontract_address
	` + filter_query

	args := make([]interface{}, len(filter_parameters))
	for i, param := range filter_parameters {
		args[i] = param
	}

	rows, err := con.Connection.Query(query, args...)
	if err != nil {
		return nil, nil, err
	}
	defer rows.Close()

	var smartcontracts []smartcontract_key.Key
	var topics []*topic.Topic

	// Loop through rows, using Scan to assign column data to struct fields.
	for rows.Next() {
		var key = smartcontract_key.Key{}
		var t topic.Topic

		if err := rows.Scan(&key.NetworkId, &key.Address, &t.Organization, &t.Project, &t.Group, &t.Smartcontract); err != nil {
			return nil, nil, err
		}
		t.NetworkId = key.NetworkId
		smartcontracts = append(smartcontracts, key)
		topics = append(topics, &t)
	}
	return smartcontracts, topics, nil
}

func GetAllFromDatabase(db *db.Database) ([]*Configuration, error) {
	rows, err := db.Connection.Query("SELECT organization, project, network_id, group_name, smartcontract_name, address FROM static_configuration WHERE 1")
	if err != nil {
		return nil, fmt.Errorf("db: %w", err)
	}

	defer rows.Close()

	configurations := make([]*Configuration, 0)

	// Loop through rows, using Scan to assign column data to struct fields.
	for rows.Next() {
		var s Configuration = Configuration{
			Topic:   topic.Topic{},
			Address: "",
		}

		if err := rows.Scan(&s.Topic.Organization, &s.Topic.Project, &s.Topic.NetworkId, &s.Topic.Group, &s.Topic.Smartcontract, &s.Address); err != nil {
			return nil, fmt.Errorf("failed to scan database result: %w", err)
		}

		configurations = append(configurations, &s)
	}
	return configurations, err
}
