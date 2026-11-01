"""Stable entities and devices across exporter restarts."""
from homeassistant.core import callback
from homeassistant.helpers.entity import DeviceInfo
from homeassistant.helpers.update_coordinator import CoordinatorEntity
from .const import DOMAIN


@callback
def setup_entities(entry, async_add_entities, entity_type, binary):
    coordinator = entry.runtime_data
    known = set()
    @callback
    def discover():
        new = []
        for key, point in coordinator.data["values"].items():
            if point["binary"] == binary and key not in known:
                known.add(key)
                new.append(entity_type(coordinator, key, point))
        async_add_entities(new)
    discover()
    entry.async_on_unload(coordinator.async_add_listener(discover))


class ExporterEntity(CoordinatorEntity):
    _attr_has_entity_name = True

    def __init__(self, coordinator, key, point):
        super().__init__(coordinator)
        self.key = key
        self._attr_unique_id = ":".join(key)
        self._attr_name = point["name"]
        device = coordinator.data["devices"].get(key[:2], {})
        self._attr_device_info = DeviceInfo(
            identifiers={(DOMAIN, ":".join(key[:2]))},
            name=device.get("deviceName") or key[1],
            model=device.get("deviceName"),
            sw_version=device.get("firmwareVersion"),
            serial_number=device.get("serialNumber"),
        )

    @property
    def available(self):
        if self.point.get("historical"):
            return True
        if not super().available or self.key not in self.coordinator.data["values"]:
            return False
        connected = self.coordinator.data["values"].get((*self.key[:2], "connected"))
        return self.key[2] == "connected" or connected is None or connected["value"]

    @property
    def point(self):
        return self.coordinator.data["values"].get(self.key, {})
