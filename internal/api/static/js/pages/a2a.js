'use strict';

function a2aPage() {
  return {
    topology: { nodes: [], edges: [] },
    events: [],
    tasks: [],
    loading: true,
    loadError: '',
    sseSource: null,
    ws: null,
    // 任务详情WebSocket连接
    taskDetailWs: null,
    showSendModal: false,
    showTaskModal: false,
    showDiscoverModal: false,
    showTaskDetailModal: false,
    selectedTask: null,
    sendFrom: '',
    sendTo: '',
    sendMsg: '',
    sendLoading: false,
    taskTitle: '',
    taskDesc: '',
    taskAssign: '',
    taskLoading: false,
    discoverUrl: '',
    discoverLoading: false,
    // 排序相关状态
    sortField: 'createdAt',
    sortDirection: 'desc',
    // 时间显示模式
    timeDisplayMode: 'relative',

    // 排序相关方法
    setSort(field) {
      if (this.sortField === field) {
        // 切换排序方向
        this.sortDirection = this.sortDirection === 'asc' ? 'desc' : 'asc';
      } else {
        // 更改排序字段，默认降序
        this.sortField = field;
        this.sortDirection = 'desc';
      }
      this.applySorting();
    },

    applySorting() {
      // 根据当前排序设置对tasks数组进行排序
      this.tasks.sort((a, b) => {
        let aVal, bVal;
        
        switch(this.sortField) {
          case 'createdAt':
            aVal = new Date(a.createdAt);
            bVal = new Date(b.createdAt);
            break;
          case 'status':
            aVal = a.status.state;
            bVal = b.status.state;
            break;
          case 'agent':
            aVal = a.agentName || a.agentId;
            bVal = b.agentName || b.agentId;
            break;
          case 'updatedAt':
            aVal = new Date(a.updatedAt);
            bVal = new Date(b.updatedAt);
            break;
          default:
            return 0;
        }
        
        if (this.sortDirection === 'asc') {
          return aVal > bVal ? 1 : (aVal < bVal ? -1 : 0);
        } else {
          return aVal < bVal ? 1 : (aVal > bVal ? -1 : 0);
        }
      });
    },

    getSortIcon(field) {
      if (this.sortField !== field) return '';
      return this.sortDirection === 'asc' ? '↑' : '↓';
    },

    // 时间显示方法
    formatTaskTime(task) {
      if (this.timeDisplayMode === 'relative') {
        return this.timeAgo(task.createdAt);
      } else {
        return this.formatDateTime(task.createdAt);
      }
    },

    async loadData() {
      this.loading = true;
      this.loadError = '';
      try {
        var results = await Promise.all([
          FangClawGoAPI.get('/api/a2a/topology'),
          FangClawGoAPI.get('/api/a2a/events?limit=200'),
          FangClawGoAPI.get('/api/tasks')
        ]);
        this.topology = results[0] || { nodes: [], edges: [] };
        this.events = results[1] || [];
        this.tasks = results[2] || [];
        
        // 应用当前排序设置
        this.applySorting();
        
        this.startSSE();
        this.startWebSocket();
      } catch(e) {
        this.loadError = e.message || 'Could not load A2A data.';
      }
      this.loading = false;
    },

    async loadTasks() {
      try {
        var newTasks = await FangClawGoAPI.get('/api/tasks') || [];
        
        // 检查是否有打开的任务详情需要保持连接
        var currentSelectedTaskId = this.selectedTask ? this.selectedTask.id : null;
        
        this.tasks = newTasks;
        // 应用当前排序设置
        this.applySorting();
        
        // 如果之前有选中的任务，尝试在新数据中找到它并保持引用
        if (currentSelectedTaskId) {
          var updatedTask = this.tasks.find(t => t.id === currentSelectedTaskId);
          if (updatedTask) {
            // 使用深拷贝和明确的赋值来确保Alpine.js检测到变化
            this.selectedTask = null;
            // 使用nextTick确保DOM更新
            setTimeout(() => {
              this.selectedTask = updatedTask;
            }, 0);
          }
        }
      } catch(e) {
        console.error('Failed to load tasks:', e);
      }
    },

    // 加载单个任务（优化：避免加载所有任务）
    async loadTask(taskId) {
      try {
        var task = await FangClawGoAPI.get('/api/tasks/' + taskId);
        return task;
      } catch(e) {
        console.error('Failed to load the task: ' + taskId, e);
        return null;
      }
    },

    startSSE() {
      if (this.sseSource) this.sseSource.close();
      var self = this;
      var url = window.location.origin + '/api/a2a/events/stream';
      if (FangClawGoAPI.getToken()) url += '?token=' + encodeURIComponent(FangClawGoAPI.getToken());
      this.sseSource = new EventSource(url);
      this.sseSource.onmessage = function(ev) {
        if (ev.data === 'ping') return;
        try {
          var event = JSON.parse(ev.data);
          if (!event.id) return;
          var exists = self.events.some(function(e) { return e.id === event.id; });
          if (!exists) {
            self.events.unshift(event);
            if (self.events.length > 200) self.events.length = 200;
          }
        } catch(e) { /* ignore parse errors */ }
      };
    },

    stopSSE() {
      if (this.sseSource) {
        this.sseSource.close();
        this.sseSource = null;
      }
    },

    startWebSocket() {
      var self = this;
      var protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:';
      var token = localStorage.getItem('fangclawgo-token') || '';
      var url = protocol + '//' + window.location.host + '/ws/a2a/tasks?token=' + encodeURIComponent(token);
      
      this.ws = new WebSocket(url);
      
      this.ws.onopen = function() {
        console.log('[A2A WS] Connected');
      };
      
      this.ws.onmessage = function(ev) {
        try {
          var msg = JSON.parse(ev.data);
          if (msg.type === 'a2a.task_status_changed') {
            var data = JSON.parse(msg.data);
            
            // 直接更新selectedTask（如果已打开且匹配）
            if (self.selectedTask && self.selectedTask.id === data.taskId && data.task) {
              // 使用双重赋值确保Alpine.js检测到变化
              self.selectedTask = null;
              setTimeout(() => {
                self.selectedTask = data.task;
              }, 0);
            }
            
            // 同时更新任务列表
            self.loadTasks();
          }
        } catch(e) {
          console.error('[A2A WS] Parse error:', e);
        }
      };
      
      this.ws.onclose = function() {
        console.log('[A2A WS] Disconnected, reconnecting in 2s...');
        setTimeout(function() {
          self.startWebSocket();
        }, 2000);
      };
      
      this.ws.onerror = function(err) {
        console.error('[A2A WS] Error:', err);
      };
    },

    stopWebSocket() {
      if (this.ws) {
        this.ws.close();
        this.ws = null;
      }
    },

    async refreshTopology() {
      try {
        this.topology = await FangClawGoAPI.get('/api/a2a/topology');
      } catch(e) { /* silent */ }
    },

    rootNodes() {
      var childIds = {};
      var self = this;
      this.topology.edges.forEach(function(e) {
        if (e.kind === 'parent_child') childIds[e.to] = true;
      });
      return this.topology.nodes.filter(function(n) { return !childIds[n.id]; });
    },

    childrenOf(id) {
      var childIds = {};
      this.topology.edges.forEach(function(e) {
        if (e.kind === 'parent_child' && e.from === id) childIds[e.to] = true;
      });
      return this.topology.nodes.filter(function(n) { return childIds[n.id]; });
    },

    peersOf(id) {
      var peerIds = {};
      this.topology.edges.forEach(function(e) {
        if (e.kind === 'peer') {
          if (e.from === id) peerIds[e.to] = true;
          if (e.to === id) peerIds[e.from] = true;
        }
      });
      return this.topology.nodes.filter(function(n) { return peerIds[n.id]; });
    },

    stateBadgeClass(state) {
      switch(state) {
        case 'Running': return 'badge badge-success';
        case 'Suspended': return 'badge badge-warning';
        case 'Terminated': case 'Crashed': return 'badge badge-danger';
        default: return 'badge badge-dim';
      }
    },

    taskStatusBadgeClass(status) {
      switch(status) {
        case 'submitted': return 'badge badge-info';
        case 'working': return 'badge badge-warning';
        case 'completed': return 'badge badge-success';
        case 'failed': return 'badge badge-danger';
        case 'cancelled': return 'badge badge-dim';
        default: return 'badge badge-dim';
      }
    },

    eventBadgeClass(kind) {
      switch(kind) {
        case 'agent_message': return 'badge badge-info';
        case 'agent_spawned': return 'badge badge-success';
        case 'agent_terminated': return 'badge badge-danger';
        case 'task_posted': return 'badge badge-warning';
        case 'task_claimed': return 'badge badge-info';
        case 'task_completed': return 'badge badge-success';
        case 'agent_discovered': return 'badge badge-info';
        default: return 'badge badge-dim';
      }
    },

    eventIcon(kind) {
      switch(kind) {
        case 'agent_message': return '\u2709';
        case 'agent_spawned': return '+';
        case 'agent_terminated': return '\u2715';
        case 'task_posted': return '\u2691';
        case 'task_claimed': return '\u2690';
        case 'task_completed': return '\u2713';
        case 'agent_discovered': return '\ud83d\udd0d';
        default: return '\u2022';
      }
    },

    eventLabel(kind) {
      switch(kind) {
        case 'agent_message': return 'Message';
        case 'agent_spawned': return 'Spawned';
        case 'agent_terminated': return 'Terminated';
        case 'task_posted': return 'Task Posted';
        case 'task_claimed': return 'Task Claimed';
        case 'task_completed': return 'Task Done';
        case 'agent_discovered': return 'Discovered';
        default: return kind;
      }
    },

    timeAgo(dateStr) {
      if (!dateStr) return '';
      var d = new Date(dateStr);
      var secs = Math.floor((Date.now() - d.getTime()) / 1000);
      if (secs < 60) return secs + 's ago';
      if (secs < 3600) return Math.floor(secs / 60) + 'm ago';
      if (secs < 86400) return Math.floor(secs / 3600) + 'h ago';
      return Math.floor(secs / 86400) + 'd ago';
    },

    formatDateTime(dateStr) {
      if (!dateStr) return '';
      var d = new Date(dateStr);
      return d.toLocaleString();
    },

    getTaskMessageText(msg) {
      if (!msg || !msg.parts) return '';
      return msg.parts.map(function(p) { return p.text || ''; }).join(' ');
    },

    openSendModal() {
      this.sendFrom = '';
      this.sendTo = '';
      this.sendMsg = '';
      this.showSendModal = true;
    },

    async submitSend() {
      if (!this.sendFrom || !this.sendTo || !this.sendMsg.trim()) return;
      this.sendLoading = true;
      try {
        var body = {
          fromAgentId: this.sendFrom,
          toAgentId: this.sendTo,
          message: this.sendMsg
        };
        var fromNode = this.topology.nodes.find(function(n) { return n.id === this.sendFrom; }.bind(this));
        if (fromNode) {
          body.fromAgentName = fromNode.name;
        }
        var toNode = this.topology.nodes.find(function(n) { return n.id === this.sendTo; }.bind(this));
        if (toNode) {
          body.toAgentName = toNode.name;
        }
        var result = await FangClawGoAPI.post('/api/comms/send', body);
        if (result && result.task) {
          this.loadTasks();
        }
        FangClawGoToast.success('Message sent');
        this.showSendModal = false;
      } catch(e) {
        FangClawGoToast.error(e.message || 'Send failed');
      }
      this.sendLoading = false;
    },

    openTaskModal() {
      this.taskTitle = '';
      this.taskDesc = '';
      this.taskAssign = '';
      this.showTaskModal = true;
    },

    async submitTask() {
      if (!this.taskTitle.trim()) return;
      this.taskLoading = true;
      try {
        var body = { title: this.taskTitle, description: this.taskDesc };
        if (this.taskAssign) {
          body.assignedTo = this.taskAssign;
          var assignedNode = this.topology.nodes.find(function(n) { return n.id === this.taskAssign; }.bind(this));
          if (assignedNode) {
            body.agentName = assignedNode.name;
          }
        }
        var result = await FangClawGoAPI.post('/api/comms/task', body);
        if (result && result.task) {
          this.loadTasks();
        }
        FangClawGoToast.success('Task posted');
        this.showTaskModal = false;
      } catch(e) {
        FangClawGoToast.error(e.message || 'Task failed');
      }
      this.taskLoading = false;
    },

    openDiscoverModal() {
      this.discoverUrl = '';
      this.showDiscoverModal = true;
    },

    async submitDiscover() {
      if (!this.discoverUrl.trim()) return;
      this.discoverLoading = true;
      try {
        await FangClawGoAPI.post('/api/a2a/discover', { url: this.discoverUrl });
        FangClawGoToast.success('Agent discovery initiated');
        this.showDiscoverModal = false;
        this.refreshTopology();
      } catch(e) {
        FangClawGoToast.error(e.message || 'Discovery failed');
      }
      this.discoverLoading = false;
    },

    async openTaskDetail(task) {
      this.showTaskDetailModal = true;
      // 立即显示传入的任务数据（避免等待）
      this.selectedTask = task;
      
      // 异步加载单个任务的最新数据（优化：避免加载所有任务）
      var latestTask = await this.loadTask(task.id);
      if (latestTask) {
        this.selectedTask = latestTask;
        // 同时更新任务列表中的对应任务
        var taskIndex = this.tasks.findIndex(t => t.id === task.id);
        if (taskIndex !== -1) {
          this.tasks[taskIndex] = latestTask;
        }
      }
    },

    closeTaskDetail() {
      this.selectedTask = null;
      this.showTaskDetailModal = false;
      // 断开任务详情WebSocket连接
      // this.disconnectTaskDetailWebSocket();
    },

    // 建立任务详情WebSocket连接
    // connectTaskDetailWebSocket(taskId) {
    //   // 先断开现有连接
    //   this.disconnectTaskDetailWebSocket();
      
    //   var protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:';
    //   // 连接到特定任务的WebSocket端点
    //   var url = protocol + '//' + window.location.host + '/ws/a2a/tasks?taskId=' + taskId;
      
    //   this.taskDetailWs = new WebSocket(url);
      
    //   this.taskDetailWs.onopen = () => {
    //     console.log('[Task Detail WS] Connected to task:', taskId);
    //   };
      
    //   this.taskDetailWs.onmessage = (ev) => {
    //     try {
    //       var msg = JSON.parse(ev.data);
    //       if (msg.type === 'a2a.task_status_changed') {
    //         var data = JSON.parse(msg.data);
    //         console.log('[Task Detail WS] Task update received:', data);
            
    //         // 精准更新selectedTask数据
    //         if (this.selectedTask && this.selectedTask.id === data.taskId) {
    //           // 更新任务状态和相关信息
    //           this.selectedTask.status = data.task.status;
    //           this.selectedTask.messages = data.task.messages;
    //           this.selectedTask.artifacts = data.task.artifacts;
    //           this.selectedTask.updatedAt = data.task.updatedAt;
              
    //           // 同时更新主任务列表中的对应任务
    //           var taskIndex = this.tasks.findIndex(t => t.id === data.taskId);
    //           if (taskIndex !== -1) {
    //             this.tasks[taskIndex] = {...this.tasks[taskIndex], ...data.task};
    //             // 重新应用排序
    //             this.applySorting();
    //           }
              
    //           console.log('[Task Detail WS] Task detail updated in real-time');
    //         }
    //       }
    //     } catch(e) {
    //       console.error('[Task Detail WS] Parse error:', e);
    //     }
    //   };
      
    //   this.taskDetailWs.onclose = () => {
    //     console.log('[Task Detail WS] Disconnected from task:', taskId);
    //     this.taskDetailWs = null;
    //   };
      
    //   this.taskDetailWs.onerror = (err) => {
    //     console.error('[Task Detail WS] Error:', err);
    //   };
    // },

    // 断开任务详情WebSocket连接
    // disconnectTaskDetailWebSocket() {
    //   if (this.taskDetailWs) {
    //     this.taskDetailWs.close();
    //     this.taskDetailWs = null;
    //   }
    // },
  };
}
