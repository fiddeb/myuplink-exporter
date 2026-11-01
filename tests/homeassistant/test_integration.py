"""Exercise real Home Assistant entities and coordinator failure/recovery."""
import asyncio
from types import MappingProxyType
from unittest.mock import AsyncMock, patch

import pytest
from homeassistant.core import HomeAssistant
from homeassistant.config_entries import ConfigEntry
from custom_components.myuplink_exporter.coordinator import ExporterCoordinator
from custom_components.myuplink_exporter.data import parse_snapshot
from custom_components.myuplink_exporter.sensor import PointSensor
from custom_components.myuplink_exporter.binary_sensor import StatusSensor
from custom_components.myuplink_exporter.entity import setup_entities
from test_data import HEADER, POINTS


def make_entry():
    return ConfigEntry(domain="myuplink_exporter", title="Test", version=1, minor_version=1,
                       data={"url": "http://exporter/probe", "max_age": 300}, options={},
                       source="user", unique_id="exporter", discovery_keys=MappingProxyType({}),
                       subentries_data=[])


@pytest.mark.asyncio
async def test_availability_and_recovery(tmp_path):
    hass = HomeAssistant(str(tmp_path))
    entry = make_entry()
    coordinator = ExporterCoordinator(hass, entry)
    snapshot = parse_snapshot(HEADER + POINTS, now=1001)
    with patch('custom_components.myuplink_exporter.coordinator.fetch_snapshot', new=AsyncMock(return_value=snapshot)):
        await coordinator.async_refresh()
    key = ('s', 'd', '62193')
    entity = PointSensor(coordinator, key, snapshot['values'][key])
    connection_key = ('s', 'd', 'connected')
    connected = StatusSensor(coordinator, connection_key, snapshot['values'][connection_key])
    assert entity.available and entity.native_value == 23
    snapshot['values'][connection_key]['value'] = False
    assert not entity.available and connected.available and not connected.is_on
    snapshot['values'][connection_key]['value'] = True
    with patch('custom_components.myuplink_exporter.coordinator.fetch_snapshot', new=AsyncMock(side_effect=ValueError('stale'))):
        await coordinator.async_refresh()
    assert not entity.available and not connected.available
    with patch('custom_components.myuplink_exporter.coordinator.fetch_snapshot', new=AsyncMock(return_value=snapshot)):
        await coordinator.async_refresh()
    assert entity.available and connected.available
    del snapshot['values'][key]
    assert not entity.available
    await hass.async_stop()


@pytest.mark.asyncio
async def test_new_points_and_no_duplicates(tmp_path):
    hass = HomeAssistant(str(tmp_path))
    entry = make_entry()
    coordinator = ExporterCoordinator(hass, entry)
    entry.runtime_data = coordinator
    snapshot = parse_snapshot(HEADER + POINTS, now=1001)
    coordinator.async_set_updated_data(snapshot)
    added = []
    setup_entities(entry, lambda entities: added.extend(entities), PointSensor, False)
    initial = len(added)
    coordinator.async_set_updated_data(snapshot)
    assert len(added) == initial
    snapshot['values']['s', 'd', 'new'] = dict(name='Temperature', value=20, unit='°C', binary=False)
    coordinator.async_set_updated_data(snapshot)
    assert len(added) == initial + 1
    assert added[-1].native_unit_of_measurement == '°C'
    assert added[-1].state_class == 'measurement'
    await coordinator.async_shutdown()
    await hass.async_stop()


@pytest.mark.asyncio
async def test_config_flow_validates_endpoint_and_rejects_stale(tmp_path):
    from unittest.mock import MagicMock
    from custom_components.myuplink_exporter.config_flow import ConfigFlow
    hass = HomeAssistant(str(tmp_path))
    flow = ConfigFlow()
    flow.hass = hass
    with patch.object(flow, 'async_set_unique_id', new=AsyncMock()) as unique, \
         patch.object(flow, '_abort_if_unique_id_configured'), \
         patch.object(flow, 'async_create_entry', return_value={'type': 'create_entry'}) as create, \
         patch('custom_components.myuplink_exporter.config_flow.fetch_snapshot', new=AsyncMock()) as fetch:
        result = await flow.async_step_user({'url': 'http://exporter:8080/', 'max_age': 300})
        assert result['type'] == 'create_entry'
        assert create.call_args.kwargs['data']['url'] == 'http://exporter:8080/probe'
        fetch.assert_awaited_once_with(hass, 'http://exporter:8080/probe', 300)
        unique.assert_awaited_once_with('http://exporter:8080/probe')
    with patch.object(flow, 'async_show_form', return_value={'type': 'form'}) as form, \
         patch('custom_components.myuplink_exporter.config_flow.fetch_snapshot', new=AsyncMock(side_effect=ValueError('stale'))):
        await flow.async_step_user({'url': 'http://exporter:8080', 'max_age': 300})
        assert form.call_args.kwargs['errors'] == {'base': 'cannot_connect'}
    await hass.async_stop()

@pytest.mark.asyncio
async def test_last_valid_return_is_separate_and_keeps_timestamp(tmp_path):
    from copy import deepcopy
    from datetime import datetime, timezone
    hass = HomeAssistant(str(tmp_path))
    coordinator = ExporterCoordinator(hass, make_entry())
    key = ('s', 'd', '62015')
    snapshot = dict(devices={('s', 'd'): {'deviceName': 'CTC EcoZenith i360'}},
                    values={key: dict(value=36, name='Return flow', unit='°C', binary=False),
                            ('s', 'd', 'connected'): dict(value=True, binary=True)},
                    timestamps={key: 950}, observed_at=1000)
    async def refresh(data):
        with patch('custom_components.myuplink_exporter.coordinator.fetch_snapshot', new=AsyncMock(return_value=deepcopy(data))):
            await coordinator.async_refresh()
    await refresh(snapshot)
    saved_key = ('s', 'd', 'return_last_valid')
    stamp_key = ('s', 'd', 'return_last_valid_timestamp')
    current = PointSensor(coordinator, key, snapshot['values'][key])
    saved = PointSensor(coordinator, saved_key, coordinator.data['values'][saved_key])
    stamp = PointSensor(coordinator, stamp_key, coordinator.data['values'][stamp_key])
    assert saved.native_value == 36 and saved.state_class is None
    assert stamp.device_class == 'timestamp'
    assert stamp.native_value == datetime.fromtimestamp(950, timezone.utc)
    del snapshot['values'][key]
    snapshot['observed_at'] = 1100
    await refresh(snapshot)
    assert not current.available and saved.available and saved.native_value == 36
    assert stamp.native_value.timestamp() == 950
    with patch('custom_components.myuplink_exporter.coordinator.fetch_snapshot', new=AsyncMock(side_effect=ValueError('stale'))):
        await coordinator.async_refresh()
    assert not current.available and saved.available
    snapshot['values'][key] = dict(value=35, name='Return flow', unit='°C', binary=False)
    snapshot['timestamps'][key] = 1150
    await refresh(snapshot)
    assert current.available and saved.native_value == 35 and stamp.native_value.timestamp() == 1150
    await coordinator.async_shutdown()
    await hass.async_stop()
