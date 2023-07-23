/*Package independent is used to scaffold the independent service
 */
package independent

import (
	"fmt"
	"github.com/ahmetson/common-lib/data_type/key_value"
	"github.com/ahmetson/service-lib/configuration"
	"github.com/ahmetson/service-lib/configuration/argument"
	"github.com/ahmetson/service-lib/context/dev"
	"github.com/ahmetson/service-lib/controller"
	"github.com/ahmetson/service-lib/log"
	"os"
	"strings"
	"sync"
)

// Service is the collection of the various Controllers
type Service struct {
	Config          *configuration.Config
	Controllers     key_value.KeyValue
	Pipelines       key_value.KeyValue
	RequiredProxies []string
	Logger          *log.Logger
}

// New service based on the configurations
func New(config *configuration.Config, logger *log.Logger) (*Service, error) {
	independent := Service{
		Config:          config,
		Logger:          logger,
		Controllers:     key_value.Empty(),
		RequiredProxies: []string{},
		Pipelines:       key_value.Empty(),
	}

	return &independent, nil
}

// AddController by their instance name
func (independent *Service) AddController(name string, controller controller.Interface) {
	independent.Controllers.Set(name, controller)
}

func (independent *Service) RequireProxy(url string) {
	independent.RequiredProxies = append(independent.RequiredProxies, url)
}

// Pipe the controller to the proxy
func (independent *Service) Pipe(proxyUrl string, name string) error {
	validProxy := false
	for _, url := range independent.RequiredProxies {
		if strings.Compare(url, proxyUrl) == 0 {
			validProxy = true
			break
		}
	}
	if !validProxy {
		return fmt.Errorf("proxy '%s' url not required. call independent.RequireProxy", proxyUrl)
	}

	if err := independent.Controllers.Exist(name); err != nil {
		return fmt.Errorf("controller instance '%s' not added. call independent.AddController: %w", name, err)
	}

	independent.Pipelines.Set(proxyUrl, name)

	return nil
}

// returns the extension urls
func (independent *Service) requiredControllerExtensions() []string {
	var extensions []string
	for _, controllerInterface := range independent.Controllers {
		c := controllerInterface.(controller.Interface)
		extensions = append(extensions, c.RequiredExtensions()...)
	}

	return extensions
}

func (independent *Service) prepareServiceConfiguration(expectedType configuration.ServiceType) error {
	// validate the independent itself
	config := independent.Config
	serviceConfig := independent.Config.Service
	if len(serviceConfig.Type) == 0 {
		exePath, err := configuration.GetCurrentPath()
		if err != nil {
			return fmt.Errorf("failed to get current executable path: %w", err)
		}

		serviceConfig = configuration.Service{
			Type:     expectedType,
			Url:      exePath,
			Instance: config.Name + " 1",
		}
	} else if serviceConfig.Type != expectedType {
		return fmt.Errorf("service type is overwritten. expected '%s', not '%s'", expectedType, serviceConfig.Type)
	}

	independent.Config.Service = serviceConfig

	return nil
}

func (independent *Service) prepareControllerConfigurations() error {
	serviceConfig := independent.Config.Service

	// validate the Controllers
	for name, controllerInterface := range independent.Controllers {
		c := controllerInterface.(controller.Interface)

		err := independent.PrepareControllerConfiguration(name, c.ControllerType())
		if err != nil {
			return fmt.Errorf("prepare '%s' controller configuration as '%s' type: %w", name, c.ControllerType(), err)
		}
	}

	independent.Config.Service = serviceConfig
	return nil
}

func (independent *Service) PrepareControllerConfiguration(name string, as configuration.Type) error {
	serviceConfig := independent.Config.Service

	// validate the Controllers
	controllerConfig, err := serviceConfig.GetController(name)
	if err == nil {
		if controllerConfig.Type != as {
			return fmt.Errorf("controller expected to be of '%s' type, not '%s'", as, controllerConfig.Type)
		}
	} else {
		controllerConfig := configuration.Controller{
			Type: as,
			Name: name,
		}

		serviceConfig.Controllers = append(serviceConfig.Controllers, controllerConfig)
	}

	err = independent.prepareInstanceConfiguration(controllerConfig)
	if err != nil {
		return fmt.Errorf("failed preparing '%s' controller instance configuration: %w", controllerConfig.Name, err)
	}

	independent.Config.Service = serviceConfig
	return nil
}

func (independent *Service) prepareInstanceConfiguration(controllerConfig configuration.Controller) error {
	serviceConfig := independent.Config.Service

	if len(controllerConfig.Instances) == 0 {
		port := independent.Config.GetFreePort()

		sourceInstance := configuration.ControllerInstance{
			Name:     controllerConfig.Name,
			Instance: controllerConfig.Name + "1",
			Port:     uint64(port),
		}
		controllerConfig.Instances = append(controllerConfig.Instances, sourceInstance)
		serviceConfig.SetController(controllerConfig)
	} else {
		if controllerConfig.Instances[0].Port == 0 {
			return fmt.Errorf("the port should not be 0 in the source")
		}
	}

	independent.Config.Service = serviceConfig
	return nil
}

// prepareConfiguration prepares yaml in service, controller, and controller instances
func (independent *Service) prepareConfiguration(expectedType configuration.ServiceType) error {
	if err := independent.prepareServiceConfiguration(expectedType); err != nil {
		return fmt.Errorf("prepareServiceConfiguration as %s: %w", expectedType, err)
	}
	serviceConfig := independent.Config.Service

	// validate the Controllers
	if err := independent.prepareControllerConfigurations(); err != nil {
		return fmt.Errorf("prepareControllerConfigurations: %w", err)
	}

	// todo validate the extensions

	independent.Config.Service = serviceConfig

	return nil
}

// preparePipelineConfiguration checks that proxy url and controllerName are valid.
// Then, in the Config, it makes sure that dependency is linted.
func (independent *Service) preparePipelineConfiguration(proxyUrl string, controllerName string) error {
	independent.Logger.Info("prepare the pipeline")

	found := false
	for _, requiredProxy := range independent.RequiredProxies {
		if strings.Compare(proxyUrl, requiredProxy) == 0 {
			found = true
			break
		}
	}
	if !found {
		return fmt.Errorf("proxy '%s' not found. add using independent.RequireProxy()", proxyUrl)
	}

	if err := independent.Controllers.Exist(controllerName); err != nil {
		return fmt.Errorf("independent.Controllers.Exist of '%s': %w", controllerName, err)
	}

	err := preparePipelineConfiguration(independent.Config, proxyUrl, controllerName, independent.Logger)

	if err != nil {
		return fmt.Errorf("service.preparePipelineConfiguration: %w", err)
	}
	return nil
}

// Prepare the services by validating, linting the configurations, as well as setting up the dependencies
func (independent *Service) Prepare(as configuration.ServiceType) error {
	if len(independent.Controllers) == 0 {
		return fmt.Errorf("no Controllers. call independent.AddController")
	}

	//
	// prepare the context for dependencies
	//---------------------------------------------------
	err := prepareContext(independent.Config.Context)
	if err != nil {
		return fmt.Errorf("service.prepareContext: %w", err)
	}

	//
	// prepare the configuration
	//----------------------------------------------------
	err = independent.prepareConfiguration(as)
	if err != nil {
		return fmt.Errorf("prepareConfiguration: %w", err)
	}

	//
	// prepare proxies
	//--------------------------------------------------
	if len(independent.RequiredProxies) > 0 {
		independent.Logger.Info("there are some proxies to setup")
		for _, requiredProxy := range independent.RequiredProxies {
			if err := prepareProxyConfiguration(requiredProxy, independent.Config, independent.Logger); err != nil {
				return fmt.Errorf("service.prepareProxyConfiguration of %s: %w", requiredProxy, err)
			}
		}

		if len(independent.Pipelines) == 0 {
			return fmt.Errorf("no pipepline to lint the proxy to the controller")
		}

		for requiredProxy, controllerInterface := range independent.Pipelines {
			controllerName := controllerInterface.(string)
			if err := independent.preparePipelineConfiguration(requiredProxy, controllerName); err != nil {
				return fmt.Errorf("preparePipelineConfiguration '%s'=>'%s': %w", requiredProxy, controllerName, err)
			}
		}
	}

	//
	// prepare extensions
	//------------------------------------------------------
	requiredExtensions := independent.requiredControllerExtensions()
	if len(requiredExtensions) > 0 {
		independent.Logger.Warn("extensions needed to be prepared", "extensions", requiredExtensions)
		for _, requiredExtension := range requiredExtensions {
			if err := prepareExtensionConfiguration(requiredExtension, independent.Config, independent.Logger); err != nil {
				return fmt.Errorf("service.prepareExtensionConfiguration of %s: %w", requiredExtension, err)
			}
		}
	}

	//
	// lint extensions, configurations to the controllers
	//---------------------------------------------------------
	for name, controllerInterface := range independent.Controllers {
		c := controllerInterface.(controller.Interface)

		controllerConfig, err := independent.Config.Service.GetController(name)
		if err != nil {
			return fmt.Errorf("c '%s' registered in the service, no config found: %w", name, err)
		}

		c.AddConfig(&controllerConfig)
		requiredExtensions := c.RequiredExtensions()
		for _, extensionUrl := range requiredExtensions {
			requiredExtension := independent.Config.Service.GetExtension(extensionUrl)
			c.AddExtensionConfig(requiredExtension)
		}
	}

	return nil
}

// BuildConfiguration creates a yaml configuration with the service parameters
func (independent *Service) BuildConfiguration() {
	if !argument.Exist(argument.BuildConfiguration) {
		return
	}
	path, err := argument.Value(argument.Path)
	if err != nil {
		independent.Logger.Fatal("requires 'path' flag", "error", err)
	}

	url, err := argument.Value(argument.Url)
	if err != nil {
		independent.Logger.Fatal("requires 'url' flag", "error", err)
	}

	independent.Config.Service.Url = url

	err = configuration.WriteService(path, independent.Config.Service)
	if err != nil {
		independent.Logger.Fatal("failed to write the proxy into the file", "error", err)
	}

	independent.Logger.Info("the proxy was generated", "path", path)

	os.Exit(0)
}

// Run the independent service.
func (independent *Service) Run() {
	independent.BuildConfiguration()

	var wg sync.WaitGroup

	for name, controllerInterface := range independent.Controllers {
		c := controllerInterface.(controller.Interface)
		if err := independent.Controllers.Exist(name); err != nil {
			independent.Logger.Fatal("controller configuration not found", "configuration", name, "error", err)
			continue
		}

		wg.Add(1)
		go func() {
			err := c.Run()
			wg.Done()
			if err != nil {
				independent.Logger.Fatal("failed to run the controller", "error", err)
			}
		}()
	}
	wg.Wait()
}

func prepareContext(context *configuration.Context) error {
	// get the extensions
	err := dev.Prepare(context)
	if err != nil {
		return fmt.Errorf("failed to prepare the context: %w", err)
	}

	return nil
}

// prepareProxyConfiguration links the proxy with the dependency.
//
// if dependency doesn't exist, it will be downloaded
func prepareProxyConfiguration(requiredProxy string, config *configuration.Config, logger *log.Logger) error {
	err := dev.PrepareConfiguration(config.Context, requiredProxy, logger)
	if err != nil {
		return fmt.Errorf("dev.PrepareConfiguration on %s: %w", requiredProxy, err)
	}

	service, err := dev.ReadServiceConfiguration(config.Context, requiredProxy)
	converted, err := configuration.ServiceToProxy(&service)
	if err != nil {
		return fmt.Errorf("proxy.ServiceToProxy: %w", err)
	}

	proxyConfiguration := config.Service.GetProxy(requiredProxy)
	if proxyConfiguration == nil {
		config.Service.SetProxy(converted)
	} else {
		if strings.Compare(proxyConfiguration.Url, converted.Url) != 0 {
			return fmt.Errorf("the proxy urls are not matching. in your configuration: %s, in the deps: %s", proxyConfiguration.Url, converted.Url)
		}
		if proxyConfiguration.Port != converted.Port {
			return fmt.Errorf("the proxy ports are not matching. in your configuration: %d, in the deps: %d", proxyConfiguration.Port, converted.Port)
		}
	}

	return nil
}

func prepareExtensionConfiguration(requiredExtension string, config *configuration.Config, logger *log.Logger) error {
	err := dev.PrepareConfiguration(config.Context, requiredExtension, logger)
	if err != nil {
		return fmt.Errorf("dev.PrepareConfiguration on %s: %w", requiredExtension, err)
	}

	service, err := dev.ReadServiceConfiguration(config.Context, requiredExtension)
	converted, err := configuration.ServiceToExtension(&service)
	if err != nil {
		return fmt.Errorf("proxy.ServiceToProxy: %w", err)
	}

	extensionConfiguration := config.Service.GetExtension(requiredExtension)
	if extensionConfiguration == nil {
		config.Service.SetExtension(converted)
	} else {
		if strings.Compare(extensionConfiguration.Url, converted.Url) != 0 {
			return fmt.Errorf("the extension url in your '%s' configuration not matches to '%s' in the dependency", extensionConfiguration.Url, converted.Url)
		}
		if extensionConfiguration.Port != extensionConfiguration.Port {
			return fmt.Errorf("your extension port '%d' not matches to '%d' port in the dependency", extensionConfiguration.Port, converted.Port)
		}
	}

	return nil
}

// preparePipelineConfiguration checks that proxy url and controllerName are valid.
// Then, in the configuration, it makes sure that dependency is linted.
func preparePipelineConfiguration(config *configuration.Config, proxyUrl string, controllerName string, logger *log.Logger) error {
	//
	// lint the dependency proxy's destination to the independent independent's controller
	//--------------------------------------------------
	proxyConfig, err := dev.ReadServiceConfiguration(config.Context, proxyUrl)
	if err != nil {
		return fmt.Errorf("dev.ReadServiceConfiguration of '%s': %w", proxyUrl, err)
	}

	destinationConfig, err := proxyConfig.GetController(configuration.DestinationName)
	if err != nil {
		return fmt.Errorf("getting dependency proxy's destination configuration failed: %w", err)
	}

	controllerConfig, err := config.Service.GetController(controllerName)
	if err != nil {
		return fmt.Errorf("getting '%s' controller from independent configuration failed: %w", controllerName, err)
	}

	// somehow it will work with only one instance. but in the future maybe another instances as well.
	destinationInstanceConfig := destinationConfig.Instances[0]
	instanceConfig := controllerConfig.Instances[0]

	if destinationInstanceConfig.Port != instanceConfig.Port {
		logger.Info("the dependency proxy destination not match to the controller",
			"proxy url", proxyUrl,
			"destination port", destinationInstanceConfig.Port,
			"independent controller port", instanceConfig.Port)

		destinationInstanceConfig.Port = instanceConfig.Port
		destinationConfig.Instances[0] = destinationInstanceConfig
		proxyConfig.SetController(destinationConfig)

		logger.Info("linting dependency proxy's destination port", "new port", instanceConfig.Port)
		logger.Warn("todo", 1, "if dependency proxy is running, then it should be restarted")
		err := dev.WriteServiceConfiguration(config.Context, proxyUrl, proxyConfig)
		if err != nil {
			return fmt.Errorf("dev.WriteServiceConfiguration for '%s': %w", proxyUrl, err)
		}
	}

	return nil
}
