package smartcontract

import (
	"fmt"

	"github.com/blocklords/gosds/common/topic"
	"github.com/blocklords/gosds/db"
)

// Whether the smartcontract address on network_id exist in database or not
func ExistInDatabase(db *db.Database, network_id string, address string) bool {
	var exists bool
	err := db.Connection.QueryRow("SELECT IF(COUNT(address),'true','false') FROM static_smartcontract WHERE network_id = ? AND address = ?", network_id, address).Scan(&exists)
	if err != nil {
		fmt.Println("Static Smartcontract exists returned db error: ", err.Error())
		return false
	}

	return exists
}

func SetInDatabase(db *db.Database, a *Smartcontract) error {
	_, err := db.Connection.Exec(`
		INSERT IGNORE INTO 
			static_smartcontract (
				network_id, 
				address, 
				abi_hash, 
				transaction_id, 
				pre_deploy_block_number, 
				pre_deploy_block_timestamp, 
				deployer
			) 
		VALUES (?, ?, ?, ?, ?, ?, ?) `,
		a.NetworkId,
		a.Address,
		a.AbiHash,
		a.Txid,
		a.PreDeployBlockNumber,
		a.PreDeployBlockTimestamp,
		a.Deployer,
	)
	if err != nil {
		fmt.Println("Failed to insert static smartcontract at network id as address", a.NetworkId, a.Address)
		return err
	}
	a.SetExists(true)
	return nil
}

// Returns the smartcontract by address on network_id from database
func GetFromDatabase(db *db.Database, network_id string, address string) (*Smartcontract, error) {
	query := `SELECT network_id, address, abi_hash, transaction_id, pre_deploy_block_number, pre_deploy_block_timestamp, deployer FROM static_smartcontract WHERE network_id = ? AND address = ?`

	var s Smartcontract

	row := db.Connection.QueryRow(query, network_id, address)
	if err := row.Scan(&s.NetworkId, &s.Address, &s.AbiHash, &s.Txid, &s.PreDeployBlockNumber, &s.PreDeployBlockTimestamp, &s.Deployer); err != nil {
		return nil, err
	}

	return &s, nil
}

// Returns the static smartcontracts by filter_parameters from database
func GetFromDatabaseFilterBy(con *db.Database, filter_query string, filter_parameters []string) ([]*Smartcontract, []*topic.Topic, error) {
	query := `SELECT s.network_id, s.address, s.abi_hash, s.transaction_id, s.pre_deploy_block_number, s.pre_deploy_block_timestamp, s.deployer,
	static_configuration.organization, static_configuration.project, static_configuration.group_name, static_configuration.smartcontract_name
	FROM static_smartcontract AS s, static_configuration WHERE
	s.network_id = static_configuration.network_id AND s.address = static_configuration.smartcontract_address
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

	var smartcontracts []*Smartcontract
	var topics []*topic.Topic

	// Loop through rows, using Scan to assign column data to struct fields.
	for rows.Next() {
		var s Smartcontract
		var t topic.Topic
		if err := rows.Scan(&s.NetworkId, &s.Address, &s.AbiHash, &s.Txid, &s.PreDeployBlockNumber, &s.PreDeployBlockTimestamp, &s.Deployer,
			&t.Organization, &t.Project, &t.Group, &t.Smartcontract); err != nil {
			return nil, nil, err
		}
		t.NetworkId = s.NetworkId
		smartcontracts = append(smartcontracts, &s)
		topics = append(topics, &t)
	}
	return smartcontracts, topics, nil
}