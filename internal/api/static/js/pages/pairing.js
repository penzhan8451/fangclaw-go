'use strict';

function pairingPage() {
  return {
    config: null,
    pairingRequest: null,
    devices: [],
    loading: true,
    loadError: '',
    pairingError: '',
    creatingPairing: false,
    removingDevice: null,
    sendingNotification: null,

    async loadDevices() {
      this.loading = true;
      this.loadError = '';
      try {
        var configData = await FangClawGoAPI.get('/api/pairing/config');
        this.config = configData;
        
        var devicesData = await FangClawGoAPI.get('/api/pairing/devices');
        this.devices = devicesData.devices || [];
      } catch(e) {
        this.loadError = e.message || 'Failed to load pairing data';
      }
      this.loading = false;
    },

    async createPairingRequest() {
      this.creatingPairing = true;
      this.pairingError = '';
      this.pairingRequest = null;
      try {
        var data = await FangClawGoAPI.post('/api/pairing/request', {});
        this.pairingRequest = data;
        this.copyToClipboard(data.token);
      } catch(e) {
        this.pairingError = e.message || 'Failed to create pairing request';
      }
      this.creatingPairing = false;
    },

    async removeDevice(deviceId) {
      if (!confirm('Are you sure you want to remove this device?')) return;
      
      this.removingDevice = deviceId;
      try {
        await FangClawGoAPI.del('/api/pairing/devices/' + encodeURIComponent(deviceId));
        FangClawGoToast.success('Device removed successfully');
        await this.loadDevices();
      } catch(e) {
        FangClawGoToast.error('Failed to remove device: ' + (e.message || 'Unknown error'));
      }
      this.removingDevice = null;
    },

    async sendNotification(deviceId) {
      this.sendingNotification = deviceId;
      try {
        await FangClawGoAPI.post('/api/pairing/notify', {
          device_id: deviceId,
          title: 'Test Notification',
          message: 'This is a test notification from FangClaw-go'
        });
        FangClawGoToast.success('Notification sent successfully');
      } catch(e) {
        FangClawGoToast.error('Failed to send notification: ' + (e.message || 'Unknown error'));
      }
      this.sendingNotification = null;
    },

    copyToClipboard(text) {
      navigator.clipboard.writeText(text).then(function() {
        FangClawGoToast.success('Token copied to clipboard');
      }).catch(function() {
        FangClawGoToast.warning('Failed to copy to clipboard');
      });
    }
  };
}
