function projectsPage() {
  return {
    projects: [],
    loading: true,
    creating: false,
    currentProject: null,
    showCreateModal: false,
    showAddMemberModal: false,
    showBindWorkflowModal: false,
    showEditWorkflowModal: false,
    newProjectName: '',
    newProjectDescription: '',
    newPMKeywords: '',
    newMemberAgentId: '',
    newMemberName: '',
    newMemberRole: '',
    chatLoading: false,
    chatMessage: '',
    availableWorkflows: [],
    bindWorkflowId: '',
    bindWorkflowName: '',
    bindTriggerMode: 'auto',
    bindKeywords: '',
    editWorkflowId: '',
    editWorkflowName: '',
    editTriggerMode: 'auto',
    editKeywords: '',
    projectTab: 'chat',
    dragOver: false,
    aiGenDescription: '',
    aiGenLoading: false,
    aiGenResult: null,
    showWorkflowCommands: false,
    selectedCommandWorkflow: null,
    hoverCommandIndex: -1,
    showProjectSidebar: false,
    cronBindings: [],
    availableCronJobs: [],
    showBindCronModal: false,
    bindCronJobId: '',
    cronResults: [],
    workflowRunHistory: [],

    async init() {
      await this.refresh();
    },

    async refresh() {
      try {
        this.loading = true;
        this.projects = await FangClawGoAPI.get('/api/projects');
      } catch (e) {
        console.error('Failed to load projects:', e);
        FangClawGoToast.error('Failed to load projects');
      } finally {
        this.loading = false;
      }
    },

    async createProject() {
      if (!this.newProjectName.trim()) {
        FangClawGoToast.error('Project name is required');
        return;
      }

      try {
        this.creating = true;
        var pmKeywords = this.newPMKeywords ? this.newPMKeywords.split(',').map(function(k) { return k.trim(); }).filter(function(k) { return k; }) : [];
        var project = await FangClawGoAPI.post('/api/projects', {
          name: this.newProjectName.trim(),
          description: this.newProjectDescription.trim(),
          pm_keywords: pmKeywords
        });

        this.projects.push(project);
        this.newProjectName = '';
        this.newProjectDescription = '';
        this.newPMKeywords = '';
        this.showCreateModal = false;
        FangClawGoToast.success('Project created successfully!');
      } catch (e) {
        console.error('Failed to create project:', e);
        FangClawGoToast.error('Failed to create project');
      } finally {
        this.creating = false;
      }
    },

    async deleteProject(projectId) {
      if (!confirm('Are you sure you want to delete this project?')) return;

      try {
        await FangClawGoAPI.delete('/api/projects/' + projectId);
        this.projects = this.projects.filter(function(p) { return p.id !== projectId; });
        if (this.currentProject && this.currentProject.id === projectId) {
          this.currentProject = null;
        }
        FangClawGoToast.success('Project deleted');
      } catch (e) {
        console.error('Failed to delete project:', e);
        FangClawGoToast.error('Failed to delete project');
      }
    },

    async selectProject(project) {
      this.currentProject = project;
      this.projectTab = 'chat';
      this.showProjectSidebar = false;  // 选中后自动关闭 sidebar
      try {
        var chat = await FangClawGoAPI.get('/api/projects/' + project.id + '/chat');
        this.currentProject.chat_history = chat;
        var updated = await FangClawGoAPI.get('/api/projects/' + project.id);
        this.currentProject.workflow_bindings = updated.workflow_bindings || [];
      } catch (e) {
        console.error('Failed to load project:', e);
      }
    },

    async addMember() {
      if (!this.newMemberAgentId || !this.newMemberName || !this.newMemberRole) {
        FangClawGoToast.error('All fields are required');
        return;
      }

      try {
        await FangClawGoAPI.post('/api/projects/' + this.currentProject.id + '/members', {
          agent_id: this.newMemberAgentId,
          name: this.newMemberName,
          role: this.newMemberRole
        });

        var updatedProject = await FangClawGoAPI.get('/api/projects/' + this.currentProject.id);
        this.currentProject = updatedProject;
        this.projects = this.projects.map(function(p) { return p.id === updatedProject.id ? updatedProject : p; });
        this.newMemberAgentId = '';
        this.newMemberName = '';
        this.newMemberRole = '';
        this.showAddMemberModal = false;
        FangClawGoToast.success('Member added');
      } catch (e) {
        console.error('Failed to add member:', e);
        FangClawGoToast.error('Failed to add member');
      }
    },

    autoFillMemberName() {
      if (this.newMemberAgentId) {
        var agents = window.Alpine ? Alpine.store('app').agents : [];
        var selectedAgent = agents.find(function(agent) { return agent.id === this.newMemberAgentId; }.bind(this));
        if (selectedAgent) {
          if (!this.newMemberName) {
            this.newMemberName = selectedAgent.name;
          }
          this.newMemberRole = this.inferRoleFromName(selectedAgent.name);
        }
      } else {
        this.newMemberRole = '';
      }
    },

    inferRoleFromName(name) {
      var nameLower = name.toLowerCase();
      var roleMap = {
        'research': 'researcher',
        'search': 'researcher',
        'analyst': 'analyst',
        'analyz': 'analyst',
        'writer': 'writer',
        'author': 'writer',
        'coder': 'coder',
        'developer': 'coder',
        'programmer': 'coder',
        'code-review': 'code-reviewer',
        'reviewer': 'code-reviewer',
        'designer': 'designer',
        'translator': 'translator'
      };
      for (var key in roleMap) {
        if (nameLower.indexOf(key) !== -1) {
          return roleMap[key];
        }
      }
      return '';
    },

    async removeMember(agentId) {
      if (!confirm('Are you sure you want to remove this member?')) return;

      try {
        await FangClawGoAPI.delete('/api/projects/' + this.currentProject.id + '/members/' + agentId);
        var updatedProject = await FangClawGoAPI.get('/api/projects/' + this.currentProject.id);
        this.currentProject = updatedProject;
        this.projects = this.projects.map(function(p) { return p.id === updatedProject.id ? updatedProject : p; });
        FangClawGoToast.success('Member removed');
      } catch (e) {
        console.error('Failed to remove member:', e);
        FangClawGoToast.error('Failed to remove member');
      }
    },

    async loadAvailableWorkflows() {
      try {
        this.availableWorkflows = await FangClawGoAPI.get('/api/workflows');
      } catch (e) {
        console.error('Failed to load workflows:', e);
        this.availableWorkflows = [];
      }
    },

    async bindWorkflow() {
      if (!this.bindWorkflowId) {
        FangClawGoToast.error('Please select a workflow');
        return;
      }

      try {
        var keywords = this.bindKeywords ? this.bindKeywords.split(',').map(function(k) { return k.trim(); }).filter(function(k) { return k; }) : [];
        await FangClawGoAPI.post('/api/projects/' + this.currentProject.id + '/workflows', {
          workflow_id: this.bindWorkflowId,
          workflow_name: this.bindWorkflowName,
          trigger_mode: this.bindTriggerMode,
          keywords: keywords
        });

        var updatedProject = await FangClawGoAPI.get('/api/projects/' + this.currentProject.id);
        this.currentProject = updatedProject;
        this.projects = this.projects.map(function(p) { return p.id === updatedProject.id ? updatedProject : p; });
        this.showBindWorkflowModal = false;
        this.bindWorkflowId = '';
        this.bindWorkflowName = '';
        this.bindTriggerMode = 'auto';
        this.bindKeywords = '';
        FangClawGoToast.success('Workflow bound to project');
      } catch (e) {
        console.error('Failed to bind workflow:', e);
        FangClawGoToast.error('Failed to bind workflow');
      }
    },

    async unbindWorkflow(workflowId) {
      if (!confirm('Unbind this workflow from the project?')) return;

      try {
        await FangClawGoAPI.delete('/api/projects/' + this.currentProject.id + '/workflows/' + encodeURIComponent(workflowId));
        var updatedProject = await FangClawGoAPI.get('/api/projects/' + this.currentProject.id);
        this.currentProject = updatedProject;
        this.projects = this.projects.map(function(p) { return p.id === updatedProject.id ? updatedProject : p; });
        FangClawGoToast.success('Workflow unbound');
      } catch (e) {
        console.error('Failed to unbind workflow:', e);
        FangClawGoToast.error('Failed to unbind workflow');
      }
    },

    async toggleWorkflowEnabled(wf) {
      try {
        await FangClawGoAPI.patch('/api/projects/' + this.currentProject.id + '/workflows/' + encodeURIComponent(wf.workflow_id), {
          enabled: !wf.enabled
        });
        var updatedProject = await FangClawGoAPI.get('/api/projects/' + this.currentProject.id);
        this.currentProject = updatedProject;
        this.projects = this.projects.map(function(p) { return p.id === updatedProject.id ? updatedProject : p; });
        FangClawGoToast.success(wf.enabled ? 'Workflow disabled' : 'Workflow enabled');
      } catch (e) {
        console.error('Failed to toggle workflow:', e);
        FangClawGoToast.error('Failed to update workflow');
      }
    },

    editWorkflowBinding(wf) {
      this.editWorkflowId = wf.workflow_id;
      this.editWorkflowName = wf.workflow_name || wf.workflow_id;
      this.editTriggerMode = wf.trigger_mode;
      this.editKeywords = (wf.keywords || []).join(', ');
      this.showEditWorkflowModal = true;
    },

    async saveWorkflowBinding() {
      try {
        var keywords = this.editKeywords ? this.editKeywords.split(',').map(function(k) { return k.trim(); }).filter(function(k) { return k; }) : [];
        await FangClawGoAPI.patch('/api/projects/' + this.currentProject.id + '/workflows/' + encodeURIComponent(this.editWorkflowId), {
          trigger_mode: this.editTriggerMode,
          keywords: keywords
        });
        var updatedProject = await FangClawGoAPI.get('/api/projects/' + this.currentProject.id);
        this.currentProject = updatedProject;
        this.projects = this.projects.map(function(p) { return p.id === updatedProject.id ? updatedProject : p; });
        this.showEditWorkflowModal = false;
        FangClawGoToast.success('Workflow binding updated');
      } catch (e) {
        console.error('Failed to update workflow binding:', e);
        FangClawGoToast.error('Failed to update workflow binding');
      }
    },

    async runWorkflow(workflowId) {
      if (!this.currentProject) return;

      try {
        var input = prompt('Enter input for the workflow (optional):');
        if (input === null || input.trim() === '') {
          return;
        }
        
        this.chatLoading = true;
        var response = await FangClawGoAPI.post('/api/projects/' + this.currentProject.id + '/workflows/' + encodeURIComponent(workflowId) + '/run', {
          content: input
        });

        this.projectTab = 'chat';
        this.currentProject.chat_history = (this.currentProject.chat_history || []).concat(response);
        var updatedProject = await FangClawGoAPI.get('/api/projects/' + this.currentProject.id);
        this.currentProject = updatedProject;
        this.projects = this.projects.map(function(p) { return p.id === updatedProject.id ? updatedProject : p; });
        FangClawGoToast.success('Workflow executed');
      } catch (e) {
        console.error('Failed to run workflow:', e);
        FangClawGoToast.error('Failed to run workflow');
      } finally {
        this.chatLoading = false;
      }
    },

    async handleWorkflowDrop(event) {
      event.preventDefault();
      this.dragOver = false;

      var file = event.dataTransfer.files[0];
      if (!file || !file.name.endsWith('.json')) {
        FangClawGoToast.error('Please drop a workflow JSON file');
        return;
      }

      try {
        var content = await file.text();
        var workflow = JSON.parse(content);

        if (!workflow.name || !workflow.steps || !workflow.steps.length) {
          FangClawGoToast.error('Invalid workflow file: missing name or steps');
          return;
        }

        if (!workflow.id) {
          workflow.id = 'wf-' + Date.now();
        }

        var registered = await FangClawGoAPI.post('/api/workflows', workflow);

        await FangClawGoAPI.post('/api/projects/' + this.currentProject.id + '/workflows', {
          workflow_id: registered.id || workflow.id,
          workflow_name: workflow.name,
          trigger_mode: 'auto',
          keywords: []
        });

        var updatedProject = await FangClawGoAPI.get('/api/projects/' + this.currentProject.id);
        this.currentProject = updatedProject;
        this.projects = this.projects.map(function(p) { return p.id === updatedProject.id ? updatedProject : p; });
        FangClawGoToast.success('Workflow uploaded and bound to project');
      } catch (e) {
        console.error('Failed to upload workflow:', e);
        FangClawGoToast.error('Failed to upload workflow: ' + (e.message || 'Unknown error'));
      }
    },

    onDragOver(event) {
      event.preventDefault();
      this.dragOver = true;
    },

    onDragLeave(event) {
      event.preventDefault();
      this.dragOver = false;
    },

    onWorkflowSelect() {
      if (this.bindWorkflowId) {
        var wf = this.availableWorkflows.find(function(w) { return w.id === this.bindWorkflowId; }.bind(this));
        if (wf) {
          this.bindWorkflowName = wf.name;
        }
      }
    },

    async sendMessage() {
      if (!this.chatMessage.trim() || !this.currentProject) return;

      try {
        this.chatLoading = true;
        var userMsg = {
          id: 'temp-' + Date.now(),
          role: 'user',
          content: this.chatMessage.trim(),
          timestamp: new Date().toISOString()
        };
        this.currentProject.chat_history = (this.currentProject.chat_history || []).concat(userMsg);

        var response = await FangClawGoAPI.post('/api/projects/' + this.currentProject.id + '/chat', {
          content: this.chatMessage.trim()
        });

        this.currentProject.chat_history = (this.currentProject.chat_history || []).concat(response);
        var updatedProject = await FangClawGoAPI.get('/api/projects/' + this.currentProject.id);
        this.currentProject = updatedProject;
        this.projects = this.projects.map(function(p) { return p.id === updatedProject.id ? updatedProject : p; });
        this.chatMessage = '';
      } catch (e) {
        console.error('Failed to send message:', e);
        FangClawGoToast.error('Failed to send message');
      } finally {
        this.chatLoading = false;
      }
    },

    formatDate(dateStr) {
      if (!dateStr) return '';
      var d = new Date(dateStr);
      return d.toLocaleString();
    },

    async generateWorkflowByAI() {
      if (!this.aiGenDescription.trim() || !this.currentProject) return;

      try {
        this.aiGenLoading = true;
        this.aiGenResult = null;
        var result = await FangClawGoAPI.post('/api/projects/' + this.currentProject.id + '/generate-workflow', {
          description: this.aiGenDescription.trim()
        });
        this.aiGenResult = result;
      } catch (e) {
        console.error('Failed to generate workflow:', e);
        FangClawGoToast.error('Failed to generate workflow: ' + (e.message || 'Unknown error'));
      } finally {
        this.aiGenLoading = false;
      }
    },

    async confirmGeneratedWorkflow() {
      if (!this.aiGenResult || !this.currentProject) return;

      try {
        var workflow = this.aiGenResult.workflow;
        if (!workflow.id) {
          workflow.id = 'wf-ai-' + Date.now();
        }

        var registered = await FangClawGoAPI.post('/api/workflows', workflow);

        await FangClawGoAPI.post('/api/projects/' + this.currentProject.id + '/workflows', {
          workflow_id: registered.id || workflow.id,
          workflow_name: workflow.name,
          trigger_mode: 'manual',
          keywords: this.aiGenResult.keywords || []
        });

        var updatedProject = await FangClawGoAPI.get('/api/projects/' + this.currentProject.id);
        this.currentProject = updatedProject;
        this.projects = this.projects.map(function(p) { return p.id === updatedProject.id ? updatedProject : p; });

        this.aiGenResult = null;
        this.aiGenDescription = '';
        FangClawGoToast.success('AI-generated workflow added to project');
      } catch (e) {
        console.error('Failed to confirm workflow:', e);
        FangClawGoToast.error('Failed to add workflow: ' + (e.message || 'Unknown error'));
      }
    },

    cancelGeneratedWorkflow() {
      this.aiGenResult = null;
    },

    onChatInput() {
      if (this.chatMessage.trim() === '/') {
        this.showWorkflowCommands = true;
        this.hoverCommandIndex = -1;
      } else {
        this.showWorkflowCommands = false;
      }
    },

    onChatKeydown(event) {
      if (!this.showWorkflowCommands) return;

      var workflows = this.currentProject.workflow_bindings || [];
      if (workflows.length === 0) return;

      switch (event.key) {
        case 'ArrowDown':
          event.preventDefault();
          this.hoverCommandIndex = (this.hoverCommandIndex + 1) % workflows.length;
          break;
        case 'ArrowUp':
          event.preventDefault();
          this.hoverCommandIndex = (this.hoverCommandIndex - 1 + workflows.length) % workflows.length;
          break;
        case 'Enter':
          event.preventDefault();
          if (this.hoverCommandIndex >= 0 && this.hoverCommandIndex < workflows.length) {
            this.selectWorkflowFromCommand(workflows[this.hoverCommandIndex]);
          }
          break;
        case 'Escape':
          event.preventDefault();
          this.showWorkflowCommands = false;
          break;
      }
    },

    selectWorkflowFromCommand(wf) {
      this.showWorkflowCommands = false;
      this.chatMessage = '';
      
      if (!confirm('Run workflow: ' + (wf.workflow_name || wf.workflow_id) + '?')) {
        return;
      }

      this.runWorkflow(wf.workflow_id);
    },

    async loadCronBindings() {
      if (!this.currentProject) return;
      try {
        this.cronBindings = await FangClawGoAPI.get('/api/projects/' + this.currentProject.id + '/crons');
      } catch (e) {
        console.error('Failed to load cron bindings:', e);
        this.cronBindings = [];
      }
      await this.loadCronResults();
    },

    async loadCronResults() {
      if (!this.currentProject) return;
      try {
        this.cronResults = await FangClawGoAPI.get('/api/projects/' + this.currentProject.id + '/crons/results');
      } catch (e) {
        console.error('Failed to load cron results:', e);
        this.cronResults = [];
      }
    },

    async loadWorkflowRunHistory() {
      try {
        this.workflowRunHistory = await FangClawGoAPI.get('/api/workflow-runs');
      } catch (e) {
        console.error('Failed to load workflow run history:', e);
        this.workflowRunHistory = [];
      }
    },

    async loadAvailableCronJobs() {
      try {
        var data = await FangClawGoAPI.get('/api/cron/jobs');
        var jobs = data.jobs || [];
        var boundIds = (this.cronBindings || []).map(function(b) { return b.job_id; });
        this.availableCronJobs = jobs.filter(function(j) {
          return boundIds.indexOf(j.id) === -1;
        });
      } catch (e) {
        console.error('Failed to load cron jobs:', e);
        this.availableCronJobs = [];
      }
    },

    getCronJobDetail(jobId) {
      return this.availableCronJobs.find(function(j) { return j.id === jobId; });
    },

    isCronAgentMember(jobId) {
      var job = this.getCronJobDetail(jobId);
      if (!job || !this.currentProject) return false;
      var actionKind = (job._raw_action && job._raw_action.kind) || job.action_kind || '';
      if (actionKind !== 'agent_turn') return true;
      var members = this.currentProject.members || [];
      return members.some(function(m) { return m.active && m.id === job.agent_id; });
    },

    formatCronSchedule(schedule) {
      if (!schedule) return '-';
      if (schedule.kind === 'cron') return schedule.expr || '-';
      if (schedule.kind === 'every') return 'every ' + schedule.every_secs + 's';
      if (schedule.kind === 'at') return 'at ' + (schedule.at || '-');
      return '-';
    },

    onCronJobSelect() {
    },

    async bindCron() {
      if (!this.bindCronJobId) {
        FangClawGoToast.error('Please select a schedule');
        return;
      }

      try {
        await FangClawGoAPI.post('/api/projects/' + this.currentProject.id + '/crons', {
          job_id: this.bindCronJobId
        });

        await this.loadCronBindings();
        this.showBindCronModal = false;
        this.bindCronJobId = '';
        FangClawGoToast.success('Schedule bound to project');
      } catch (e) {
        console.error('Failed to bind cron:', e);
        FangClawGoToast.error('Failed to bind schedule: ' + (e.message || 'Unknown error'));
      }
    },

    async unbindCron(jobId) {
      if (!confirm('Unbind this schedule from the project?')) return;

      try {
        await FangClawGoAPI.delete('/api/projects/' + this.currentProject.id + '/crons/' + encodeURIComponent(jobId));
        await this.loadCronBindings();
        FangClawGoToast.success('Schedule unbound');
      } catch (e) {
        console.error('Failed to unbind cron:', e);
        FangClawGoToast.error('Failed to unbind schedule');
      }
    },

    async toggleCronBindingEnabled(cb) {
      try {
        await FangClawGoAPI.patch('/api/projects/' + this.currentProject.id + '/crons/' + encodeURIComponent(cb.job_id), {
          enabled: !cb.enabled
        });
        await this.loadCronBindings();
        FangClawGoToast.success(cb.enabled ? 'Schedule disabled' : 'Schedule enabled');
      } catch (e) {
        console.error('Failed to toggle cron binding:', e);
        FangClawGoToast.error('Failed to update schedule');
      }
    },

    async addCronAgentAsMember(cb) {
      try {
        var data = await FangClawGoAPI.get('/api/cron/jobs');
        var jobs = data.jobs || [];
        var job = jobs.find(function(j) { return j.id === cb.job_id; });
        if (!job) {
          FangClawGoToast.error('Cron job not found');
          return;
        }

        var agents = window.Alpine ? Alpine.store('app').agents : [];
        var agent = agents.find(function(a) { return a.id === job.agent_id; });
        var agentName = agent ? agent.name : job.agent_id;
        var role = this.inferRoleFromName(agentName) || 'member';

        await FangClawGoAPI.post('/api/projects/' + this.currentProject.id + '/members', {
          agent_id: job.agent_id,
          name: agentName,
          role: role
        });

        var updatedProject = await FangClawGoAPI.get('/api/projects/' + this.currentProject.id);
        this.currentProject = updatedProject;
        this.projects = this.projects.map(function(p) { return p.id === updatedProject.id ? updatedProject : p; });
        await this.loadCronBindings();
        FangClawGoToast.success('Agent added as member');
      } catch (e) {
        console.error('Failed to add agent as member:', e);
        FangClawGoToast.error('Failed to add agent as member');
      }
    }
  };
}
