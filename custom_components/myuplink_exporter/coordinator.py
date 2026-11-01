"""One local poll for all entities."""
from datetime import datetime, timedelta, timezone
import asyncio
import logging
import aiohttp
from homeassistant.helpers.aiohttp_client import async_get_clientsession
from homeassistant.helpers.update_coordinator import DataUpdateCoordinator, UpdateFailed
from .data import parse_snapshot


async def fetch_snapshot(hass, url, max_age):
    async with asyncio.timeout(15):
        async with async_get_clientsession(hass).get(url, allow_redirects=False) as response:
            if response.status != 200:
                raise ValueError("Exporter did not return a successful snapshot")
            return parse_snapshot(await response.text(), max_age)


class ExporterCoordinator(DataUpdateCoordinator):
    def __init__(self, hass, entry):
        super().__init__(hass, logging.getLogger(__name__), name="myUplink Exporter", config_entry=entry,
                         update_interval=timedelta(seconds=30))
        self.entry = entry
        self.last_valid_return = {}

    async def _async_update_data(self):
        try:
            snapshot = await fetch_snapshot(self.hass, self.entry.data["url"], self.entry.data["max_age"])
            for device_key, device in snapshot["devices"].items():
                if not device.get("deviceName", "").startswith("CTC EcoZenith"):
                    continue
                key = (*device_key, "62015")
                point = snapshot["values"].get(key)
                connected = snapshot["values"].get((*device_key, "connected"), {}).get("value", False)
                if connected and point and isinstance(point["value"], (int, float)):
                    timestamp = snapshot.get("timestamps", {}).get(key, snapshot["observed_at"])
                    self.last_valid_return[device_key] = (dict(point), timestamp)
                if device_key in self.last_valid_return:
                    saved, timestamp = self.last_valid_return[device_key]
                    snapshot["values"][(*device_key, "return_last_valid")] = dict(
                        saved, name="Return last valid", historical=True)
                    snapshot["values"][(*device_key, "return_last_valid_timestamp")] = dict(
                        value=datetime.fromtimestamp(timestamp, timezone.utc),
                        name="Return last valid timestamp", binary=False, historical=True,
                        device_class="timestamp")
            return snapshot
        except (aiohttp.ClientError, TimeoutError, ValueError) as err:
            raise UpdateFailed("Exporter unavailable, invalid or stale") from err
