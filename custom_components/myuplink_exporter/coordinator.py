"""One local poll for all entities."""
from datetime import timedelta
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

    async def _async_update_data(self):
        try:
            return await fetch_snapshot(self.hass, self.entry.data["url"], self.entry.data["max_age"])
        except (aiohttp.ClientError, TimeoutError, ValueError) as err:
            raise UpdateFailed("Exporter unavailable, invalid or stale") from err
