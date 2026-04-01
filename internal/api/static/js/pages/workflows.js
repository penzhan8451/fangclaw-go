// FangClaw-go Workflows Page — Workflow builder + run history
'use strict';

function workflowsPage() {
  return {
    // -- Workflows state --
    workflows: [],
    agents: [],
    showCreateModal: false,
    showEditModal: false,
    editingWf: null,
    runModal: null,
    runInput: '',
    runResult: '',
    running: false,
    loading: true,
    loadError: '',
    newWf: { name: '', description: '', steps: [{ name: '', agent_name: '', mode: 'sequential', prompt: '', condition: '', max_iterations: 5, until: '', error_mode: 'fail', max_retries: 3 }] },

    // -- Workflows methods --
    async loadWorkflows() {
      this.loading = true;
      this.loadError = '';
      try {
        this.workflows = await FangClawGoAPI.get('/api/workflows');
      } catch(e) {
        this.workflows = [];
        this.loadError = e.message || 'Could not load workflows.';
      }
      this.loading = false;
    },

    async loadAgents() {
      try {
        var list = await FangClawGoAPI.get('/api/agents');
        this.agents = Array.isArray(list) ? list : [];
      } catch(e) {
        this.agents = [];
      }
    },

    async loadData() { 
      await Promise.all([this.loadWorkflows(), this.loadAgents()]);
    },

    async createWorkflow() {
      var steps = this.newWf.steps.map(function(s) {
        var step = { name: s.name || 'step', agent_name: s.agent_name, mode: s.mode, prompt: s.prompt || '{{input}}', error_mode: s.error_mode || 'fail' };
        if (s.mode === 'conditional') {
          step.condition = s.condition || '';
        } else if (s.mode === 'loop') {
          if (s.max_iterations) {
            step.max_iterations = parseInt(s.max_iterations, 10) || 5;
          }
          if (s.until) {
            step.until = s.until;
          }
        }
        if (s.error_mode === 'retry' && s.max_retries) {
          step.max_retries = parseInt(s.max_retries, 10) || 3;
        }
        return step;
      });
      try {
        var wfName = this.newWf.name;
        await FangClawGoAPI.post('/api/workflows', { name: wfName, description: this.newWf.description, steps: steps });
        this.showCreateModal = false;
        this.newWf = { name: '', description: '', steps: [{ name: '', agent_name: '', mode: 'sequential', prompt: '', condition: '', max_iterations: 5, until: '', error_mode: 'fail', max_retries: 3 }] };
        FangClawGoToast.success('Workflow "' + wfName + '" created');
        await this.loadWorkflows();
      } catch(e) {
        FangClawGoToast.error('Failed to create workflow: ' + e.message);
      }
    },

    async openEditWorkflow(wf) {
      try {
        var fullWf = await FangClawGoAPI.get('/api/workflows/' + wf.id);
        var copy = JSON.parse(JSON.stringify(fullWf));
        if (Array.isArray(copy.steps)) {
          copy.steps = copy.steps.map(function(s) {
            return {
              name: s.name || '',
              agent_name: (s.agent && s.agent.name) || '',
              mode: (s.mode && s.mode.type) || 'sequential',
              prompt: s.prompt_template || '',
              condition: (s.mode && s.mode.condition) || '',
              max_iterations: (s.mode && s.mode.max_iterations) || 5,
              until: (s.mode && s.mode.until) || '',
              error_mode: (s.error_mode && s.error_mode.type) || 'fail',
              max_retries: (s.error_mode && s.error_mode.max_retries) || 3
            };
          });
        } else {
          copy.steps = [{ name: '', agent_name: '', mode: 'sequential', prompt: '', condition: '', max_iterations: 5, until: '', error_mode: 'fail', max_retries: 3 }];
        }
        this.editingWf = copy;
        this.showEditModal = true;
      } catch(e) {
        FangClawGoToast.error('Failed to load workflow: ' + e.message);
      }
    },

    async saveWorkflow() {
      if (!Array.isArray(this.editingWf.steps)) {
        FangClawGoToast.error('Invalid workflow steps');
        return;
      }
      var steps = this.editingWf.steps.map(function(s) {
        var step = { name: s.name || 'step', agent_name: s.agent_name, mode: s.mode, prompt: s.prompt || '{{input}}', error_mode: s.error_mode || 'fail' };
        if (s.mode === 'conditional') {
          step.condition = s.condition || '';
        } else if (s.mode === 'loop') {
          if (s.max_iterations) {
            step.max_iterations = parseInt(s.max_iterations, 10) || 5;
          }
          if (s.until) {
            step.until = s.until;
          }
        }
        if (s.error_mode === 'retry' && s.max_retries) {
          step.max_retries = parseInt(s.max_retries, 10) || 3;
        }
        return step;
      });
      try {
        var wfName = this.editingWf.name;
        await FangClawGoAPI.put('/api/workflows/' + this.editingWf.id, { name: wfName, description: this.editingWf.description, steps: steps });
        this.showEditModal = false;
        this.editingWf = null;
        FangClawGoToast.success('Workflow "' + wfName + '" saved');
        await this.loadWorkflows();
      } catch(e) {
        FangClawGoToast.error('Failed to save workflow: ' + e.message);
      }
    },

    showRunModal(wf) {
      this.runModal = wf;
      this.runInput = '';
      this.runResult = '';
    },

    async executeWorkflow() {
      if (!this.runModal) return;
      this.running = true;
      this.runResult = '';
      try {
        var res = await FangClawGoAPI.post('/api/workflows/' + this.runModal.id + '/run', { input: this.runInput });
        this.runResult = res.output || JSON.stringify(res, null, 2);
        FangClawGoToast.success('Workflow completed');
      } catch(e) {
        this.runResult = 'Error: ' + e.message;
        FangClawGoToast.error('Workflow failed: ' + e.message);
      }
      this.running = false;
    },

    async viewRuns(wf) {
      try {
        var runs = await FangClawGoAPI.get('/api/workflows/' + wf.id + '/runs');
        this.runResult = JSON.stringify(runs, null, 2);
        this.runModal = wf;
      } catch(e) {
        FangClawGoToast.error('Failed to load run history: ' + e.message);
      }
    },

    async deleteWorkflow(wf) {
      var self = this;
      FangClawGoToast.confirm('Delete Workflow', 'Delete "' + wf.name + '"? This cannot be undone.', async function() {
        try {
          await FangClawGoAPI.del('/api/workflows/' + wf.id);
          self.workflows = self.workflows.filter(function(w) { return w.id !== wf.id; });
          FangClawGoToast.success('Workflow "' + wf.name + '" deleted');
        } catch(e) {
          FangClawGoToast.error('Failed to delete workflow: ' + (e.message || e));
        }
      });
    }
  };
}
