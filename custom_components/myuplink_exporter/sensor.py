"""Numeric points and enum states; no guessed energy counters."""
from homeassistant.components.sensor import SensorEntity, SensorDeviceClass, SensorStateClass
from .entity import ExporterEntity, setup_entities

# Only units with unambiguous semantics receive a device/state class.
UNITS = {"°C": SensorDeviceClass.TEMPERATURE, "°F": SensorDeviceClass.TEMPERATURE,
         "W": SensorDeviceClass.POWER, "kW": SensorDeviceClass.POWER,
         "V": SensorDeviceClass.VOLTAGE, "A": SensorDeviceClass.CURRENT,
         "Hz": SensorDeviceClass.FREQUENCY, "bar": SensorDeviceClass.PRESSURE}


async def async_setup_entry(hass, entry, async_add_entities):
    setup_entities(entry, async_add_entities, PointSensor, False)


class PointSensor(ExporterEntity, SensorEntity):
    @property
    def native_value(self):
        return self.point.get("value")

    @property
    def native_unit_of_measurement(self):
        return self.point.get("unit") or None

    @property
    def device_class(self):
        return self.point.get("device_class") or UNITS.get(self.native_unit_of_measurement)

    @property
    def state_class(self):
        return SensorStateClass.MEASUREMENT if self.device_class and not self.point.get("historical") else None
