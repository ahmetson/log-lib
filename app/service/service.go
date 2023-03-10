package service

import (
	"fmt"

	"github.com/blocklords/sds/app/configuration"
	"github.com/blocklords/sds/security/credentials"
)

// Environment variables for each SDS Service
type Service struct {
	Name          string // Service name
	Credentials   *credentials.Credentials
	inproc        bool
	url           string
	broadcast_url string
}

// Creates the service with the parameters but without any information
func Inprocess(service_type ServiceType) (*Service, error) {
	name := string(service_type)

	s := Service{
		Name:          name,
		inproc:        true,
		url:           "inproc://reply_" + name,
		broadcast_url: "inproc://pub_" + name,
	}

	return &s, nil
}

// Creates the service with the parameters but without any information
func NewExternal(service_type ServiceType, limit Limit) (*Service, error) {
	default_configuration := DefaultConfiguration(service_type)
	app_config := configuration.NewService(default_configuration)

	name := string(service_type)
	host_env := name + "_HOST"
	port_env := name + "_PORT"
	broadcast_host_env := name + "_BROADCAST_HOST"
	broadcast_port_env := name + "_BROADCAST_PORT"

	s := Service{
		Name:        name,
		inproc:      false,
		Credentials: nil,
	}

	switch limit {
	case REMOTE:
		s.url = fmt.Sprintf("tcp://%s:%s", app_config.GetString(host_env), app_config.GetString(port_env))
	case THIS:
		s.url = fmt.Sprintf("tcp://*:%s", app_config.GetString(port_env))
	case SUBSCRIBE:
		s.broadcast_url = fmt.Sprintf("tcp://%s:%s", app_config.GetString(broadcast_host_env), app_config.GetString(broadcast_port_env))
	case BROADCAST:
		s.broadcast_url = fmt.Sprintf("tcp://*:%s", app_config.GetString(broadcast_port_env))
	}

	return &s, nil
}

// Creates the service with the parameters that includes
// private and private key
func NewSecure(service_type ServiceType, limit Limit) (*Service, error) {
	default_configuration := DefaultConfiguration(service_type)
	app_config := configuration.NewService(default_configuration)

	name := string(service_type)
	public_key := name + "_PUBLIC_KEY"
	broadcast_public_key := name + "_BROADCAST_PUBLIC_KEY"

	s, err := NewExternal(service_type, limit)
	if err != nil {
		return nil, fmt.Errorf("service.New: %w", err)
	}

	switch limit {
	case REMOTE:
		if !app_config.Exist(public_key) {
			return nil, fmt.Errorf("security enabled, but missing %s", s.Name)
		}
		s.Credentials = credentials.New(public_key)
	case THIS:
		bucket, key_name := service_type.SecretKeyPath()

		creds, err := credentials.NewFromVault(bucket, key_name)
		if err != nil {
			return nil, fmt.Errorf("vault.GetString for %s service secret key: %w", s.Name, err)
		}

		s.Credentials = creds
	case SUBSCRIBE:
		if !app_config.Exist(broadcast_public_key) {
			return nil, fmt.Errorf("security enabled, but missing %s", s.Name)
		}

		s.Credentials = credentials.New(app_config.GetString(broadcast_public_key))
	case BROADCAST:
		bucket, key_name := service_type.BroadcastSecretKeyPath()

		creds, err := credentials.NewFromVault(bucket, key_name)
		if err != nil {
			return nil, fmt.Errorf("vault.GetString for %s service secret key: %w", s.Name, err)
		}

		s.Credentials = creds
	}

	return s, nil
}

// Returns the request-reply url as a host:port
func (e *Service) Url() string {
	return e.url
}

// Returns the broadcast url as a host:port
func (e *Service) BroadcastUrl() string {
	return e.broadcast_url
}
