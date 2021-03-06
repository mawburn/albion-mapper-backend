package albion

import (
	"encoding/json"
	"io/ioutil"
	"log"
	"strings"
	"time"

	"github.com/tebro/albion-mapper-backend/db"
)

// Resource describes a resource node
type Resource struct {
	Name string `json:"name"`
	Tier string `json:"tier"`
}

// Zone describes a map in albion
type Zone struct {
	Name      string     `json:"name"`
	Color     string     `json:"color"`
	Type      string     `json:"type"`
	Markers   []string   `json:"markers"`
	Resources []Resource `json:"resources"`
}

var validColors = []string{"black", "red", "yellow", "blue", "road"}
var zones = []Zone{}

var markerMapping = map[string]string{
	"roads_of_avalon_solo_pve":  "Solo PvE content",
	"roads_of_avalon_group_pve": "Group PvE content",
	"roads_of_avalon_raid_pve":  "Raid PvE content",
}

type dataResources struct {
	Name string `json:"@name"`
	Tier string `json:"@tier"`
}

type mapMarker struct {
	Type string `json:"@type"`
}

type dataZone struct {
	Name      string          `json:"name"`
	Type      string          `json:"type"`
	Resources []dataResources `json:"resources"`
	Markers   []mapMarker     `json:"markers"`
}

// LoadZones reads the "data-dump.json" file and loads it into memory
func LoadZones() error {
	dat, err := ioutil.ReadFile("data-dump.json")
	if err != nil {
		return err
	}
	var raw []dataZone
	err = json.Unmarshal(dat, &raw)
	if err != nil {
		return err
	}
	for _, z := range raw {
		zone := Zone{Name: z.Name, Type: z.Type}

		if z.Type == "SAFEAREA" {
			zone.Color = "blue"
		} else {
			parts := strings.Split(z.Type, "_")
			if parts[0] == "TUNNEL" {
				zone.Color = "road"
			}

			if parts[0] == "OPENPVP" {
				zone.Color = strings.ToLower(parts[1])
			}
		}

		for _, marker := range z.Markers {
			name := marker.Type
			if mappedName, ok := markerMapping[name]; ok {
				name = mappedName
			}
			zone.Markers = append(zone.Markers, name)
		}

		for _, r := range z.Resources {
			zone.Resources = append(zone.Resources, Resource{Name: r.Name, Tier: r.Tier})
		}

		zones = append(zones, zone)
	}

	return nil
}

// Portal describes a roads portal between two zones
type Portal struct {
	id       int
	Source   string    `json:"source"`
	Target   string    `json:"target"`
	Size     int       `json:"size"`
	Expires  time.Time `json:"expires"`
	TimeLeft float64   `json:"timeLeft"`
}

func zoneNameInZones(name string, zones []Zone) bool {
	for _, z := range zones {
		if z.Name == name {
			return true
		}
	}
	return false
}

// IsValidPortal checks that the portal is OK
func IsValidPortal(portal Portal) (bool, error) {
	if !(portal.Size == 2 || portal.Size == 7 || portal.Size == 20) {
		return false, nil
	}

	zones := GetZones()

	return zoneNameInZones(portal.Source, zones) && zoneNameInZones(portal.Target, zones), nil
}

// GetZones returns all zones in the DB
func GetZones() []Zone {
	return zones
}

// GetPortals returns the portals in the DB
func GetPortals() ([]Portal, error) {
	db, err := db.GetDb()
	if err != nil {
		return []Portal{}, err
	}
	rows, err := db.Query("SELECT id, source, target, size, expires from portals;")
	if err != nil {
		return []Portal{}, err
	}
	defer rows.Close()

	portals := []Portal{}
	for rows.Next() {
		var portal Portal
		var expires []uint8
		err = rows.Scan(&portal.id, &portal.Source, &portal.Target, &portal.Size, &expires)
		if err != nil {
			return []Portal{}, err
		}

		portal.Expires, err = time.Parse("2006-01-02 15:04:05", string(expires))
		if err != nil {
			return []Portal{}, err
		}

		now := time.Now()
		portal.TimeLeft = portal.Expires.Sub(now).Minutes()
		portals = append(portals, portal)
	}

	return portals, nil
}

// AddPortal adds a new portal to the DB
func AddPortal(portal Portal) error {
	db, err := db.GetDb()
	if err != nil {
		return err
	}
	q, err := db.Query("REPLACE INTO portals (source, target, size, expires) VALUES (?, ?, ?, ?);", portal.Source, portal.Target, portal.Size, portal.Expires)
	defer q.Close()

	return err
}

func deletePortal(portal Portal) error {
	db, err := db.GetDb()
	if err != nil {
		return nil
	}

	q, err := db.Query("DELETE FROM portals WHERE id = ?;", portal.id)
	defer q.Close()

	return err
}

// CleanupExpiredPortals does what you think
func CleanupExpiredPortals() error {
	portals, err := GetPortals()
	if err != nil {
		return err
	}

	toDelete := []Portal{}
	now := time.Now()

	for _, p := range portals {
		if p.Expires.Before(now) {
			toDelete = append(toDelete, p)
		}
	}

	for _, p := range toDelete {
		err = deletePortal(p)
		if err != nil {
			log.Printf("Unable to delete portal: %v", err)
		}
	}
	return err
}
