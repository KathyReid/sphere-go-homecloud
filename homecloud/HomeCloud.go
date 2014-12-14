package homecloud

import (
	"reflect"
	"sync"
	"time"

	"github.com/ninjasphere/go-ninja/api"
	"github.com/ninjasphere/go-ninja/config"
	"github.com/ninjasphere/go-ninja/logger"
	"github.com/ninjasphere/go-ninja/model"
	"github.com/ninjasphere/redigo/redis"
	"github.com/ninjasphere/sphere-go-homecloud/models"
)

var log = logger.GetLogger("HomeCloud")

type HomeCloud struct {
	Conn         *ninja.Connection    `inject:""`
	Pool         *redis.Pool          `inject:""`
	ThingModel   *models.ThingModel   `inject:""`
	DeviceModel  *models.DeviceModel  `inject:""`
	ChannelModel *models.ChannelModel `inject:""`
	RoomModel    *models.RoomModel    `inject:""`
	ModuleModel  *models.ModuleModel  `inject:""`
	SiteModel    *models.SiteModel    `inject:""`
	log          *logger.Logger
}

func (c *HomeCloud) PostConstruct() error {

	c.log = logger.GetLogger("HomeCloud")

	c.ExportRPCServices()
	c.ensureSiteExists()

	// \/ \/ This is terrible \/ \/
	ledController := c.Conn.GetServiceClient("$node/" + config.Serial() + "/led-controller")
	go func() {
		for {
			err := ledController.Call("enableControl", nil, nil, time.Second*5)
			if err != nil {
				c.log.Infof("Failed to enable control on LED controller: %s", err)
				time.Sleep(time.Second * 2)
			} else {
				time.Sleep(time.Second * 20)
			}
		}
	}()

	// We wait for at least one sync to happen, or fail
	<-c.StartSyncing(config.MustDuration("homecloud.sync.interval"))

	c.AutoStartModules()

	return nil
}

// if config.Bool(false, "clearcloud") {

func (c *HomeCloud) ClearCloud() {
	log.Infof("Clearing all cloud data in 5 seconds")

	time.Sleep(time.Second * 5)

	c.ThingModel.ClearCloud()
	c.ChannelModel.ClearCloud()
	c.DeviceModel.ClearCloud()
	c.RoomModel.ClearCloud()
	c.SiteModel.ClearCloud()

	log.Infof("All cloud data cleared? Probably.")
}

func (c *HomeCloud) ExportRPCServices() {
	c.Conn.MustExportService(c.ThingModel, "$home/services/ThingModel", &model.ServiceAnnouncement{
		Schema: "/service/thing-model",
	})
	c.Conn.MustExportService(c.DeviceModel, "$home/services/DeviceModel", &model.ServiceAnnouncement{
		Schema: "/service/device-model",
	})
	c.Conn.MustExportService(c.RoomModel, "$home/services/RoomModel", &model.ServiceAnnouncement{
		Schema: "/service/room-model",
	})
	c.Conn.MustExportService(c.SiteModel, "$home/services/SiteModel", &model.ServiceAnnouncement{
		Schema: "/service/site-model",
	})
}

type syncable interface {
	Sync(time.Duration) error
}

func (c *HomeCloud) StartSyncing(interval time.Duration) chan bool {

	syncComplete := make(chan bool)

	syncTimeout := config.MustDuration("homecloud.sync.timeout")
	syncModels := []syncable{c.RoomModel, c.DeviceModel, c.ChannelModel, c.ThingModel, c.ThingModel}

	go func() {
		for {

			c.log.Infof("\n\n\n------ Timed model syncing started (every %s) ------ ", interval.String())

			var wg sync.WaitGroup

			wg.Add(len(syncModels))

			success := true

			for _, model := range syncModels {
				go func(model syncable) {
					err := model.Sync(syncTimeout)
					if err != nil {
						c.log.Warningf("Failed to sync model %s : %s", reflect.TypeOf(model).String(), err)
						success = false
					}
					wg.Done()
				}(model)
			}

			wg.Wait()

			log.Infof("------ Timed model syncing complete. Success? %t ------\n\n\n", success)

			select {
			case syncComplete <- success:
			default:
			}

			time.Sleep(interval)
		}
	}()

	return syncComplete
}

func (c *HomeCloud) ensureSiteExists() {
	site, err := c.SiteModel.Fetch(config.MustString("siteId"))
	if err != nil && err != models.RecordNotFound {
		log.Fatalf("Failed to get site: %s", err)
	}

	if err == models.RecordNotFound {
		siteType := "home"
		name := "Home"
		site = &model.Site{
			Name: &name,
			ID:   config.MustString("siteId"),
			Type: &siteType,
		}
		err = c.SiteModel.Create(site)
		if err != nil && err != models.RecordNotFound {
			log.Fatalf("Failed to create site: %s", err)
		}
	}

}

func (c *HomeCloud) AutoStartModules() {

	do := func(name string, task string) error {
		return c.Conn.SendNotification("$node/"+config.Serial()+"/module/"+task, name)
	}

	interval := config.MustDuration("homecloud.autoStart.interval")

	for _, name := range config.MustStringArray("homecloud.autoStart.modules") {
		log.Infof("-- (Re)starting '%s'", name)

		err := do(name, "stop")
		if err != nil {
			log.Fatalf("Failed to send %s stop message! %s", name, err)
		}

		time.Sleep(interval)

		err = do(name, "start")
		if err != nil {
			log.Fatalf("Failed to send %s start message! %s", name, err)
		}
	}

}
