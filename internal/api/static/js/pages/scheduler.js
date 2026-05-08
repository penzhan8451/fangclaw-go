// FangClaw-go Scheduler Page — Cron job management + event triggers unified view
'use strict';

function schedulerPage() {
  return {
    tab: 'jobs',

    // -- Scheduled Jobs state --
    jobs: [],
    loading: true,
    loadError: '',

    // -- Event Triggers state --
    triggers: [],
    trigLoading: false,
    trigLoadError: '',

    // -- Run History state --
    history: [],
    historyLoading: false,

    // -- Trigger History state --
    triggerHistory: [],
    triggerHistoryLoading: false,

    // -- Create Job form --
    showCreateForm: false,
    newJob: {
      name: '',
      cron: '',
      action_kind: 'agent_turn',
      agent_id: '',
      message: '',
      event_text: '',
      shell_command: '',
      shell_args: '',
      enabled: true,
      delivery_kind: 'last_channel',
      delivery_channel_name: '',
      delivery_recipient: '',
      delivery_webhook_url: ''
    },
    creating: false,

    // -- Edit Job form --
    showEditForm: false,
    editingJobId: '',
    editJob: {
      name: '',
      cron: '',
      action_kind: 'agent_turn',
      agent_id: '',
      message: '',
      event_text: '',
      shell_command: '',
      shell_args: '',
      enabled: true,
      delivery_kind: 'none',
      delivery_channel_name: '',
      delivery_recipient: '',
      delivery_webhook_url: ''
    },
    editing: false,

    // -- Create Trigger form --
    showCreateTriggerForm: false,
    newTrigger: {
      agent_id: '',
      pattern_type: 'all',
      name_pattern: '',
      keyword: '',
      key_pattern: '',
      substring: '',
      prompt_template: 'Event: {{event}}',
      max_fires: 0
    },
    creatingTrigger: false,

    // -- Run Now state --
    runningJobId: '',

    // -- Channels for delivery --
    channels: [],

    // -- Shell security config --
    shellSecurity: null,

    // Cron presets
    cronPresets: [
      { label: 'Every minute', cron: '* * * * *' },
      { label: 'Every 5 minutes', cron: '*/5 * * * *' },
      { label: 'Every 15 minutes', cron: '*/15 * * * *' },
      { label: 'Every 30 minutes', cron: '*/30 * * * *' },
      { label: 'Every hour', cron: '0 * * * *' },
      { label: 'Every 6 hours', cron: '0 */6 * * *' },
      { label: 'Daily at midnight', cron: '0 0 * * *' },
      { label: 'Daily at 9am', cron: '0 9 * * *' },
      { label: 'Weekdays at 9am', cron: '0 9 * * 1-5' },
      { label: 'Every Monday 9am', cron: '0 9 * * 1' },
      { label: 'First of month', cron: '0 0 1 * *' }
    ],

    // ── Lifecycle ──

    async loadData() {
      this.loading = true;
      this.loadError = '';
      try {
        await this.loadJobs();
        await this.loadChannels();
        await this.loadShellSecurity();
      } catch(e) {
        this.loadError = e.message || 'Could not load scheduler data.';
      }
      this.loading = false;
    },

    async loadJobs() {
      var data = await FangClawGoAPI.get('/api/cron/jobs');
      var raw = data.jobs || [];
      // Normalize cron API response to flat fields the UI expects
      this.jobs = raw.map(function(j) {
        var cron = '';
        if (j.schedule) {
          if (j.schedule.kind === 'cron') cron = j.schedule.expr || '';
          else if (j.schedule.kind === 'every') cron = 'every ' + j.schedule.every_secs + 's';
          else if (j.schedule.kind === 'at') cron = 'at ' + (j.schedule.at || '');
        }
        return {
          id: j.id,
          name: j.name,
          cron: cron,
          agent_id: j.agent_id,
          message: j.action ? j.action.message || '' : '',
          enabled: j.enabled,
          last_run: j.last_run,
          next_run: j.next_run,
          delivery: j.delivery ? j.delivery.kind || '' : '',
          _raw_delivery: j.delivery || null,
          _raw_action: j.action || null,
          created_at: j.created_at
        };
      });
    },

    async loadTriggers() {
      this.trigLoading = true;
      this.trigLoadError = '';
      try {
        var data = await FangClawGoAPI.get('/api/triggers');
        this.triggers = Array.isArray(data) ? data : [];
      } catch(e) {
        this.triggers = [];
        this.trigLoadError = e.message || 'Could not load triggers.';
      }
      this.trigLoading = false;
    },

    async loadChannels() {
      try {
        var data = await FangClawGoAPI.get('/api/channels');
        this.channels = Array.isArray(data) ? data : [];
      } catch(e) {
        this.channels = [];
      }
    },

    async loadShellSecurity() {
      try {
        this.shellSecurity = await FangClawGoAPI.get('/api/cron/shell-security');
      } catch(e) {
        this.shellSecurity = null;
      }
    },

    async loadHistory() {
      this.historyLoading = true;
      try {
        var historyItems = [];
        var jobs = this.jobs || [];
        for (var i = 0; i < jobs.length; i++) {
          var job = jobs[i];
          if (job.last_run) {
            historyItems.push({
              timestamp: job.last_run,
              name: job.name || '(unnamed)',
              type: 'schedule',
              status: 'completed',
              run_count: 0
            });
          }
        }
        var triggers = this.triggers || [];
        for (var j = 0; j < triggers.length; j++) {
          var t = triggers[j];
          if (t.fire_count > 0) {
            historyItems.push({
              timestamp: t.created_at,
              name: 'Trigger: ' + this.triggerType(t.pattern, t.pattern_type),
              type: 'trigger',
              status: 'fired',
              run_count: t.fire_count
            });
          }
        }
        historyItems.sort(function(a, b) {
          return new Date(b.timestamp).getTime() - new Date(a.timestamp).getTime();
        });
        this.history = historyItems;
      } catch(e) {
        this.history = [];
      }
      this.historyLoading = false;
    },

    async loadTriggerHistory() {
      this.triggerHistoryLoading = true;
      try {
        var data = await FangClawGoAPI.get('/api/trigger-history?limit=100');
        this.triggerHistory = Array.isArray(data) ? data : [];
      } catch(e) {
        this.triggerHistory = [];
      }
      this.triggerHistoryLoading = false;
    },

    // ── Job CRUD ──

    async createJob() {
      if (!this.newJob.name.trim()) {
        FangClawGoToast.warn('Please enter a job name');
        return;
      }
      if (!this.newJob.cron.trim()) {
        FangClawGoToast.warn('Please enter a cron expression');
        return;
      }
      this.creating = true;
      try {
        var jobName = this.newJob.name;
        var action = { kind: this.newJob.action_kind };
        if (this.newJob.action_kind === 'agent_turn') {
          action.message = this.newJob.message || 'Scheduled task: ' + this.newJob.name;
        } else if (this.newJob.action_kind === 'system_event') {
          if (!this.newJob.event_text.trim()) {
            FangClawGoToast.warn('Please enter event text');
            this.creating = false;
            return;
          }
          action.text = this.newJob.event_text;
        } else if (this.newJob.action_kind === 'execute_shell') {
          if (!this.newJob.shell_command.trim()) {
            FangClawGoToast.warn('Please enter a command');
            this.creating = false;
            return;
          }
          action.command = this.newJob.shell_command;
          if (this.newJob.shell_args.trim()) {
            action.args = this.newJob.shell_args.trim().split(/\s+/);
          }
        }
        var delivery = { kind: this.newJob.delivery_kind };
        if (this.newJob.delivery_kind === 'channel') {
          delivery.channel_name = this.newJob.delivery_channel_name;
          if (this.newJob.delivery_recipient) delivery.recipient = this.newJob.delivery_recipient;
        } else if (this.newJob.delivery_kind === 'webhook') {
          delivery.url = this.newJob.delivery_webhook_url;
        }
        var body = {
          agent_id: this.newJob.action_kind === 'agent_turn' ? this.newJob.agent_id : '',
          name: this.newJob.name,
          schedule: { kind: 'cron', expr: this.newJob.cron },
          action: action,
          delivery: delivery,
          enabled: this.newJob.enabled
        };
        await FangClawGoAPI.post('/api/cron/jobs', body);
        this.showCreateForm = false;
        this.newJob = { name: '', cron: '', action_kind: 'agent_turn', agent_id: '', message: '', event_text: '', shell_command: '', shell_args: '', enabled: true, delivery_kind: 'last_channel', delivery_channel_name: '', delivery_recipient: '', delivery_webhook_url: '' };
        FangClawGoToast.success('Schedule "' + jobName + '" created');
        await this.loadJobs();
      } catch(e) {
        FangClawGoToast.error('Failed to create schedule: ' + (e.message || e));
      }
      this.creating = false;
    },

    async toggleJob(job) {
      try {
        var newState = !job.enabled;
        await FangClawGoAPI.put('/api/cron/jobs/' + job.id + '/enable', { enabled: newState });
        job.enabled = newState;
        FangClawGoToast.success('Schedule ' + (newState ? 'enabled' : 'paused'));
      } catch(e) {
        FangClawGoToast.error('Failed to toggle schedule: ' + (e.message || e));
      }
    },

    deleteJob(job) {
      var self = this;
      var jobName = job.name || job.id;
      FangClawGoToast.confirm('Delete Schedule', 'Delete "' + jobName + '"? This cannot be undone.', async function() {
        try {
          await FangClawGoAPI.del('/api/cron/jobs/' + job.id);
          self.jobs = self.jobs.filter(function(j) { return j.id !== job.id; });
          FangClawGoToast.success('Schedule "' + jobName + '" deleted');
        } catch(e) {
          FangClawGoToast.error('Failed to delete schedule: ' + (e.message || e));
        }
      });
    },

    async runNow(job) {
      this.runningJobId = job.id;
      try {
        var result = await FangClawGoAPI.post('/api/schedules/' + job.id + '/run', {});
        console.log("Schedules: Run Now Result:", result);
        
        if (result.status === 'started' || result.status === 'completed') {
          FangClawGoToast.success('Schedule "' + (job.name || 'job') + '" execution started');
          job.last_run = new Date().toISOString();
        } else {
          FangClawGoToast.error('Schedule run failed: ' + (result.error || 'Unknown error'));
        }
      } catch(e) {
        FangClawGoToast.error('Run Now is not yet available for cron jobs');
      }
      this.runningJobId = '';
    },

    openEditForm(job) {
      this.editingJobId = job.id;
      var deliveryKind = job.delivery || 'none';
      var channelName = '';
      var recipient = '';
      var webhookUrl = '';
      if (job._raw_delivery) {
        channelName = job._raw_delivery.channel_name || '';
        recipient = job._raw_delivery.recipient || '';
        webhookUrl = job._raw_delivery.url || '';
      }
      var actionKind = 'agent_turn';
      var message = job.message || '';
      var eventText = '';
      var shellCommand = '';
      var shellArgs = '';
      if (job._raw_action) {
        actionKind = job._raw_action.kind || 'agent_turn';
        if (job._raw_action.message) message = job._raw_action.message;
        if (job._raw_action.text) eventText = job._raw_action.text;
        if (job._raw_action.command) shellCommand = job._raw_action.command;
        if (job._raw_action.args && job._raw_action.args.length) shellArgs = job._raw_action.args.join(' ');
      }
      this.editJob = {
        name: job.name || '',
        cron: job.cron || '',
        action_kind: actionKind,
        agent_id: job.agent_id || '',
        message: message,
        event_text: eventText,
        shell_command: shellCommand,
        shell_args: shellArgs,
        enabled: job.enabled !== false,
        delivery_kind: deliveryKind,
        delivery_channel_name: channelName,
        delivery_recipient: recipient,
        delivery_webhook_url: webhookUrl
      };
      this.showEditForm = true;
    },

    async updateJob() {
      if (!this.editJob.name.trim()) {
        FangClawGoToast.error('Job name is required');
        return;
      }
      if (!this.editJob.cron.trim()) {
        FangClawGoToast.error('Cron expression is required');
        return;
      }

      this.editing = true;
      try {
        var action = { kind: this.editJob.action_kind };
        if (this.editJob.action_kind === 'agent_turn') {
          action.message = this.editJob.message || 'Scheduled task: ' + this.editJob.name;
        } else if (this.editJob.action_kind === 'system_event') {
          action.text = this.editJob.event_text;
        } else if (this.editJob.action_kind === 'execute_shell') {
          action.command = this.editJob.shell_command;
          if (this.editJob.shell_args.trim()) {
            action.args = this.editJob.shell_args.trim().split(/\s+/);
          }
        }
        var delivery = { kind: this.editJob.delivery_kind };
        if (this.editJob.delivery_kind === 'channel') {
          delivery.channel_name = this.editJob.delivery_channel_name;
          if (this.editJob.delivery_recipient) delivery.recipient = this.editJob.delivery_recipient;
        } else if (this.editJob.delivery_kind === 'webhook') {
          delivery.url = this.editJob.delivery_webhook_url;
        }
        await FangClawGoAPI.put('/api/schedules/' + this.editingJobId, {
          agent_id: this.editJob.action_kind === 'agent_turn' ? this.editJob.agent_id : '',
          name: this.editJob.name,
          enabled: this.editJob.enabled,
          schedule: {
            kind: 'cron',
            expr: this.editJob.cron
          },
          action: action,
          delivery: delivery
        });
        FangClawGoToast.success('Schedule updated successfully');
        this.showEditForm = false;
        await this.loadData();
      } catch(e) {
        console.error("Failed to update schedule:", e);
        FangClawGoToast.error('Failed to update schedule');
      }
      this.editing = false;
    },

    // ── Trigger helpers ──

    triggerType(pattern, patternType) {
      var names = {
        lifecycle: 'Lifecycle',
        agent_spawned: 'Agent Spawn',
        agent_terminated: 'Agent Terminated',
        system: 'System',
        system_keyword: 'System Keyword',
        memory_update: 'Memory Update',
        memory_key_pattern: 'Memory Key',
        all: 'All Events',
        content_match: 'Content Match'
      };
      if (patternType && names[patternType]) {
        return names[patternType];
      }
      if (!pattern) return 'unknown';
      if (typeof pattern === 'string') return pattern;
      var keys = Object.keys(pattern);
      if (keys.length === 0) return 'unknown';
      var key = keys[0];
      return names[key] || key.replace(/_/g, ' ');
    },

    async toggleTrigger(trigger) {
      try {
        var newState = !trigger.enabled;
        await FangClawGoAPI.put('/api/triggers/' + trigger.id, { enabled: newState });
        trigger.enabled = newState;
        FangClawGoToast.success('Trigger ' + (newState ? 'enabled' : 'disabled'));
      } catch(e) {
        FangClawGoToast.error('Failed to toggle trigger: ' + (e.message || e));
      }
    },

    deleteTrigger(trigger) {
      var self = this;
      FangClawGoToast.confirm('Delete Trigger', 'Delete this trigger? This cannot be undone.', async function() {
        try {
          await FangClawGoAPI.del('/api/triggers/' + trigger.id);
          self.triggers = self.triggers.filter(function(t) { return t.id !== trigger.id; });
          FangClawGoToast.success('Trigger deleted');
        } catch(e) {
          FangClawGoToast.error('Failed to delete trigger: ' + (e.message || e));
        }
      });
    },

    async createTrigger() {
      if (!this.newTrigger.agent_id.trim()) {
        FangClawGoToast.warn('Please select an agent');
        return;
      }
      if (!this.newTrigger.prompt_template.trim()) {
        FangClawGoToast.warn('Please enter a prompt template');
        return;
      }
      this.creatingTrigger = true;
      try {
        var pattern = { type: this.newTrigger.pattern_type };
        if (this.newTrigger.pattern_type === 'agent_spawned' && this.newTrigger.name_pattern) {
          pattern.name_pattern = this.newTrigger.name_pattern;
        } else if (this.newTrigger.pattern_type === 'system_keyword' && this.newTrigger.keyword) {
          pattern.keyword = this.newTrigger.keyword;
        } else if (this.newTrigger.pattern_type === 'memory_key_pattern' && this.newTrigger.key_pattern) {
          pattern.key_pattern = this.newTrigger.key_pattern;
        } else if (this.newTrigger.pattern_type === 'content_match' && this.newTrigger.substring) {
          pattern.substring = this.newTrigger.substring;
        }
        
        var body = {
          agent_id: this.newTrigger.agent_id,
          pattern: pattern,
          prompt_template: this.newTrigger.prompt_template,
          max_fires: this.newTrigger.max_fires || 0
        };
        await FangClawGoAPI.post('/api/triggers', body);
        this.showCreateTriggerForm = false;
        this.newTrigger = {
          agent_id: '',
          pattern_type: 'all',
          name_pattern: '',
          keyword: '',
          key_pattern: '',
          substring: '',
          prompt_template: 'Event: {{event}}',
          max_fires: 0
        };
        FangClawGoToast.success('Trigger created successfully');
        await this.loadTriggers();
      } catch(e) {
        FangClawGoToast.error('Failed to create trigger: ' + (e.message || e));
      }
      this.creatingTrigger = false;
    },

    // ── Utility ──

    get agents() {
      return Alpine.store('app').agents || [];
    },

    get availableAgents() {
      return Alpine.store('app').agents || [];
    },

    agentName(agentId) {
      if (!agentId) return '(any)';
      var agents = this.availableAgents;
      for (var i = 0; i < agents.length; i++) {
        if (agents[i].id === agentId) return agents[i].name;
      }
      if (agentId.length > 12) return agentId.substring(0, 8) + '...';
      return agentId;
    },

    describeCron(expr) {
      if (!expr) return '';
      // Handle non-cron schedule descriptions
      if (expr.indexOf('every ') === 0) return expr;
      if (expr.indexOf('at ') === 0) return 'One-time: ' + expr.substring(3);

      var map = {
        '* * * * *': 'Every minute',
        '*/2 * * * *': 'Every 2 minutes',
        '*/5 * * * *': 'Every 5 minutes',
        '*/10 * * * *': 'Every 10 minutes',
        '*/15 * * * *': 'Every 15 minutes',
        '*/30 * * * *': 'Every 30 minutes',
        '0 * * * *': 'Every hour',
        '0 */2 * * *': 'Every 2 hours',
        '0 */4 * * *': 'Every 4 hours',
        '0 */6 * * *': 'Every 6 hours',
        '0 */12 * * *': 'Every 12 hours',
        '0 0 * * *': 'Daily at midnight',
        '0 6 * * *': 'Daily at 6:00 AM',
        '0 9 * * *': 'Daily at 9:00 AM',
        '0 12 * * *': 'Daily at noon',
        '0 18 * * *': 'Daily at 6:00 PM',
        '0 9 * * 1-5': 'Weekdays at 9:00 AM',
        '0 9 * * 1': 'Mondays at 9:00 AM',
        '0 0 * * 0': 'Sundays at midnight',
        '0 0 1 * *': '1st of every month',
        '0 0 * * 1': 'Mondays at midnight'
      };
      if (map[expr]) return map[expr];

      var parts = expr.split(' ');
      if (parts.length !== 5) return expr;

      var min = parts[0];
      var hour = parts[1];
      var dom = parts[2];
      var mon = parts[3];
      var dow = parts[4];

      if (min.indexOf('*/') === 0 && hour === '*' && dom === '*' && mon === '*' && dow === '*') {
        return 'Every ' + min.substring(2) + ' minutes';
      }
      if (min === '0' && hour.indexOf('*/') === 0 && dom === '*' && mon === '*' && dow === '*') {
        return 'Every ' + hour.substring(2) + ' hours';
      }

      var dowNames = { '0': 'Sun', '1': 'Mon', '2': 'Tue', '3': 'Wed', '4': 'Thu', '5': 'Fri', '6': 'Sat', '7': 'Sun',
                       '1-5': 'Weekdays', '0,6': 'Weekends', '6,0': 'Weekends' };

      if (dom === '*' && mon === '*' && min.match(/^\d+$/) && hour.match(/^\d+$/)) {
        var h = parseInt(hour, 10);
        var m = parseInt(min, 10);
        var ampm = h >= 12 ? 'PM' : 'AM';
        var h12 = h === 0 ? 12 : (h > 12 ? h - 12 : h);
        var mStr = m < 10 ? '0' + m : '' + m;
        var timeStr = h12 + ':' + mStr + ' ' + ampm;
        if (dow === '*') return 'Daily at ' + timeStr;
        var dowLabel = dowNames[dow] || ('DoW ' + dow);
        return dowLabel + ' at ' + timeStr;
      }

      return expr;
    },

    applyCronPreset(preset) {
      this.newJob.cron = preset.cron;
    },

    formatTime(ts) {
      if (!ts) return '-';
      try {
        var d = new Date(ts);
        if (isNaN(d.getTime())) return '-';
        return d.toLocaleString();
      } catch(e) { return '-'; }
    },

    relativeTime(ts) {
      if (!ts) return 'never';
      try {
        var diff = Date.now() - new Date(ts).getTime();
        if (isNaN(diff)) return 'never';
        if (diff < 0) {
          // Future time
          var absDiff = Math.abs(diff);
          if (absDiff < 60000) return 'in <1m';
          if (absDiff < 3600000) return 'in ' + Math.floor(absDiff / 60000) + 'm';
          if (absDiff < 86400000) return 'in ' + Math.floor(absDiff / 3600000) + 'h';
          return 'in ' + Math.floor(absDiff / 86400000) + 'd';
        }
        if (diff < 60000) return 'just now';
        if (diff < 3600000) return Math.floor(diff / 60000) + 'm ago';
        if (diff < 86400000) return Math.floor(diff / 3600000) + 'h ago';
        return Math.floor(diff / 86400000) + 'd ago';
      } catch(e) { return 'never'; }
    },

    jobCount() {
      var enabled = 0;
      for (var i = 0; i < this.jobs.length; i++) {
        if (this.jobs[i].enabled) enabled++;
      }
      return enabled;
    },

    triggerCount() {
      var enabled = 0;
      for (var i = 0; i < this.triggers.length; i++) {
        if (this.triggers[i].enabled) enabled++;
      }
      return enabled;
    }
  };
}
