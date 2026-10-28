const auth = require('./utils/auth');
const agreement = require('./utils/agreement');

App({
  globalData: {
    ready: false,
    user: null,
  },

  onLaunch() {
    // 合规顺序：先确认用户已自主同意协议，再进入功能。
    // 未同意前不进入任何功能页，也不发起任何登录/数据请求（规范 3.4.1）。
    if (!agreement.hasConsented()) {
      // onLaunch 阶段页面栈尚未就绪，延后一帧再跳转更稳妥
      setTimeout(() => {
        wx.reLaunch({ url: '/pages/agreement/agreement' });
      }, 0);
      return;
    }
    // 已同意：仅恢复本地会话，不主动发起登录授权，
    // 是否登录由用户在首页自行选择（避免"未体验功能即要求授权"）。
    const u = auth.getUser();
    if (u) this.globalData.user = u;
    this.globalData.ready = true;
    // 冷启动：若入口页本身已是确认页（小程序码 page=pages/web-login/web-login），
    // 由该页 onLoad 处理 scene，这里不再路由，避免重复替换页面导致重复绑定。
    const lo = (wx.getLaunchOptionsSync && wx.getLaunchOptionsSync()) || {};
    if (lo.path !== 'pages/web-login/web-login') {
      this.routeScene(lo.query || {});
    }
  },

  onShow(options) {
    // 小程序码/扫码进入：warm-start 时微信会把新的 scene 通过 onShow 传入，
    // 转发到网页授权确认页统一处理（bind:<token> 或 12 位登录票号）。
    if (!agreement.hasConsented()) return;
    // 冷启动时 App.onShow 先于页面 onLoad 触发，此时页面栈为空；
    // 该场景已由 onLaunch / 入口页 onLoad 处理，跳过以避免重复路由。
    if (getCurrentPages().length === 0) return;
    const q = (options && options.query) || {};
    this.routeScene(q);
  },

  // 将小程序码携带的 scene 路由到确认页；避免重复跳转当前已在确认页的情况
  routeScene(q) {
    const scene = q.scene || '';
    const isBind = scene.startsWith('bind:');
    const isTicket = /^[A-Za-z0-9]{12}$/.test(scene);
    if (!isBind && !isTicket) return;
    const pages = getCurrentPages();
    const top = pages[pages.length - 1];
    if (top && top.route === 'pages/web-login/web-login') return;
    wx.reLaunch({ url: '/pages/web-login/web-login?scene=' + encodeURIComponent(scene) });
  },
});
