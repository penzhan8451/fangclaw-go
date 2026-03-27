'use strict';

function deliveryPage() {
  return {
    activeTab: 'receipts',
    deliveryReceipts: [],
    deliveryTasks: [],
    agents: [],
    selectedAgent: '',
    statusFilter: '',
    loadingReceipts: false,
    loadingTasks: false,
    loadingAgents: false,
    receiptsError: '',
    tasksError: '',
    agentsError: '',
    receiptsLimit: 200,

    async init() {
      await this.loadAgents();
      await this.loadDeliveryReceipts();
      await this.loadDeliveryTasks();
    },

    async loadAgents() {
      this.loadingAgents = true;
      this.agentsError = '';
      try {
        var data = await FangClawGoAPI.get('/api/agents');
        this.agents = Array.isArray(data) ? data : [];
      } catch(e) {
        this.agentsError = e.message || 'Failed to load agents';
      }
      this.loadingAgents = false;
    },

    getAgentName(agentId) {
      var agent = this.agents.find(a => a.id === agentId);
      return agent ? agent.name : agentId;
    },

    async loadDeliveryReceipts() {
      this.loadingReceipts = true;
      this.receiptsError = '';
      this.deliveryReceipts = [];
      try {
        if (this.selectedAgent && this.selectedAgent !== '') {
          var data = await FangClawGoAPI.get('/api/agents/' + encodeURIComponent(this.selectedAgent) + '/deliveries?limit=' + this.receiptsLimit);
          this.deliveryReceipts = data.receipts || [];
        } else {
          var data = await FangClawGoAPI.get('/api/deliveries/receipts?limit=' + this.receiptsLimit);
          this.deliveryReceipts = data.receipts || [];
        }
      } catch(e) {
        this.receiptsError = e.message || 'Failed to load delivery receipts';
      }
      this.loadingReceipts = false;
    },

    async loadDeliveryTasks() {
      this.loadingTasks = true;
      this.tasksError = '';
      try {
        var data = await FangClawGoAPI.get('/api/deliveries');
        this.deliveryTasks = data.deliveries || [];
      } catch(e) {
        this.tasksError = e.message || 'Failed to load delivery tasks';
      }
      this.loadingTasks = false;
    },

    get filteredReceipts() {
      if (!this.statusFilter) return this.deliveryReceipts;
      return this.deliveryReceipts.filter(r => r.status === this.statusFilter);
    },

    get filteredTasks() {
      if (!this.statusFilter) return this.deliveryTasks;
      return this.deliveryTasks.filter(t => t.status === this.statusFilter);
    },

    get totalReceipts() {
      return this.deliveryReceipts.length;
    },

    get sentCount() {
      return this.deliveryReceipts.filter(r => r.status === 'sent' || r.status === 'delivered').length;
    },

    get failedCount() {
      return this.deliveryReceipts.filter(r => r.status === 'failed').length;
    },

    get bestEffortCount() {
      return this.deliveryReceipts.filter(r => r.status === 'best_effort').length;
    },

    get pendingTasksCount() {
      return this.deliveryTasks.filter(t => t.status === 'pending').length;
    },

    get inProgressTasksCount() {
      return this.deliveryTasks.filter(t => t.status === 'in_progress').length;
    },

    get doneTasksCount() {
      return this.deliveryTasks.filter(t => t.status === 'done').length;
    },

    getStatusBadgeClass(status) {
      switch(status) {
        case 'sent':
        case 'delivered':
          return 'badge-success';
        case 'failed':
          return 'badge-error';
        case 'best_effort':
          return 'badge-muted';
        case 'pending':
          return 'badge-warn';
        case 'in_progress':
          return 'badge-accent';
        case 'done':
          return 'badge-success';
        default:
          return 'badge-muted';
      }
    },

    refreshReceipts() {
      this.loadDeliveryReceipts();
    },

    refreshTasks() {
      this.loadDeliveryTasks();
    },

    refreshAll() {
      this.loadDeliveryReceipts();
      this.loadDeliveryTasks();
    }
  };
}
