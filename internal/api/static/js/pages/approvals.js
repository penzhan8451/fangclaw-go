// FangClaw-go Approvals Page — Execution approval queue for sensitive agent actions
'use strict';

function approvalsPage() {
  return {
    approvals: [],
    filterStatus: 'all',
    loading: true,
    loadError: '',
    initialized: false,

    get filtered() {
      var f = this.filterStatus;
      if (f === 'all') return this.approvals;
      return this.approvals.filter(function(a) { return a.status === f; });
    },

    get pendingCount() {
      return this.approvals.filter(function(a) { return a.status === 'pending'; }).length;
    },

    async init() {
      var self = this;
      if(!Alpine.store('app').currentUser) {
        this.loading = false;
        // 监听 user-login 事件，确保用户登录后加载数据
        document.addEventListener('user-login', async function() {
          await self.loadData();
        });
        return;
      }
      
      this.loading = true;
      this.loadError = '';
      try {
        await self.loadData();
      } catch(e) {
        self.loadError = e.message || 'Could not load approvals.';
      } finally {
        self.loading = false;
      }
      // 监听 user-logout 事件，清空数据
      document.addEventListener('user-logout', function() {
        self.approvals = [];
        self.loadError = '';
        self.loading = false;
      });
      
      // 尝试立即加载数据
      // var tryLoad = async function() {
      //   if (Alpine.store('app') && Alpine.store('app').currentUser) {
      //     await self.loadData();
      //   } else {
      //     // 如果还没准备好，先设置 loading 为 false，稍后再试
      //     self.loading = false;
      //     // 延迟一小段时间后再次尝试
      //     setTimeout(function() {
      //       if (Alpine.store('app') && Alpine.store('app').currentUser) {
      //         self.loadData();
      //       }
      //     }, 500);
      //   }
      // };
      
      // // 检查 Alpine store 是否已经准备好
      // if (Alpine.store) {
      //   await tryLoad();
      // } else {
      //   // 如果 Alpine 还没完全初始化，等待 alpine:init 事件
      //   document.addEventListener('alpine:init', async function() {
      //     await tryLoad();
      //   });
      //   self.loading = false;
      // }
      
      self.initialized = true;
    },

    async loadData() {
      if (!Alpine.store('app').currentUser) {
        this.loading = false;
        return;
      }
      this.loading = true;
      this.loadError = '';
      try {
        var data = await FangClawGoAPI.get('/api/approvals');
        this.approvals = data.approvals || [];
      } catch(e) {
        this.loadError = e.message || 'Could not load approvals.';
      }
      this.loading = false;
    },

    async approve(id) {
      var self = this;
      try {
        // Find the approval to get session_id
        var approval = this.approvals.find(function(a) { return a.id === id; });
      
        await FangClawGoAPI.post('/api/approvals/' + id + '/approve', {});
        FangClawGoToast.success('Approved');
        
        // Don't auto-navigate - stay on approvals page for multi-user scenario
        // Navigate back to chat if we have session_id
        // if (approval && approval.session_id) {
        //   var store = Alpine.store('app');
        //   
        //   // Use the agent info directly from the approval object
        //   var agent = {
        //     id: approval.agent_id,
        //     name: approval.agent_name || approval.agent_id,
        //     model_provider: approval.model_provider || '?',
        //     model_name: approval.model_name || '?'
        //   };
        //   
        //   store.pendingAgent = agent;
        //   store.pendingSession = approval.session_id;
        //   // console.log('[Approvals] Navigating to chat with session:', approval.session_id, 'agent:', agent);
        //   
        //   // Navigate to agents page
        //   location.hash = 'agents';
        // } else {
        await this.loadData();
        // }
      } catch(e) {
        FangClawGoToast.error(e.message);
      }
    },

    async reject(id) {
      var self = this;
      FangClawGoToast.confirm('Reject Action', 'Are you sure you want to reject this action?', async function() {
        try {
          // Find the approval to get session_id
          var approval = self.approvals.find(function(a) { return a.id === id; });
          console.log('[Approvals] Reject:', approval);
          
          await FangClawGoAPI.post('/api/approvals/' + id + '/reject', {});
          FangClawGoToast.success('Rejected');
          
          // Don't auto-navigate - stay on approvals page for multi-user scenario
          // Navigate back to chat if we have session_id
          // if (approval && approval.session_id) {
          //   var store = Alpine.store('app');
          //   
          //   // Use the agent info directly from the approval object
          //   var agent = {
          //     id: approval.agent_id,
          //     name: approval.agent_name || approval.agent_id,
          //     model_provider: approval.model_provider || '?',
          //     model_name: approval.model_name || '?'
          //   };
          //   
          //   store.pendingAgent = agent;
          //   store.pendingSession = approval.session_id;
          //   console.log('[Approvals] Navigating to chat with session:', approval.session_id, 'agent:', agent);
          //   
          //   // Navigate to agents page
          //   location.hash = 'agents';
          // } else {
          await self.loadData();
          // }
        } catch(e) {
          FangClawGoToast.error(e.message);
        }
      });
    },

    timeAgo(dateStr) {
      if (!dateStr) return '';
      var d = new Date(dateStr);
      var secs = Math.floor((Date.now() - d.getTime()) / 1000);
      if (secs < 60) return secs + 's ago';
      if (secs < 3600) return Math.floor(secs / 60) + 'm ago';
      if (secs < 86400) return Math.floor(secs / 3600) + 'h ago';
      return Math.floor(secs / 86400) + 'd ago';
    }
  };
}
