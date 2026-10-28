const { request } = require('../../utils/request');
const auth = require('../../utils/auth');
const scan = require('../../utils/scan');

// 网页登录确认（方案 B：小程序码携带 scene）
// 两种场景，均复用本确认页，按 scene 前缀分流：
//   1) 网页登录：scene = 网页登录票号 → 自动 confirm（/api/auth/web-login/confirm）
//   2) 网页「绑定微信」：scene = bind:<token> → 扫码即绑（/api/auth/wechat/bind/confirm）
// 此外仍支持从「我的」页手动进入后输入网页票号。
// 后端契约：
//   网页登录  POST /api/auth/web-login/confirm      {ticket, code}        code=0 即成功
//   扫码即绑  POST /api/auth/wechat/bind/confirm    {bind_token, code}    code=0 即成功
//   失败：41009 票号/令牌过期或已使用；41020 该微信尚未注册账号
Page({
  data: {
    ticket: '',
    status: '',
    submitting: false,
  },

  onLoad(query) {
    const raw = (query && query.scene) || '';
    let scene = raw;
    try { scene = decodeURIComponent(raw); } catch (e) { /* 非编码串则原样使用 */ }
    if (!scene) return;
    // 网页「绑定微信」小程序码：scene = bind:<token>，扫码即绑，无需手动输入
    if (scene.startsWith('bind:')) {
      this.bindWechat(scene.slice(5));
      return;
    }
    // 否则视为网页登录票号
    this.setData({ ticket: scene });
    this.confirm();
  },

  onInput(e) {
    this.setData({ ticket: e.detail.value });
  },

  submit() {
    this.confirm();
  },

  confirm() {
    if (this.data.submitting) return;
    // 后端票据统一转大写存储，客户端同步处理，避免大小写不一致
    const ticket = (this.data.ticket || '').trim().toUpperCase();
    if (!ticket) {
      wx.showToast({ title: '请输入网页显示的票号', icon: 'none' });
      return;
    }

    this.setData({ ticket: ticket, submitting: true, status: '确认中…' });
    auth.wxLoginCode()
      .then((code) => request({
        url: '/api/auth/web-login/confirm',
        method: 'POST',
        data: { ticket: ticket, code: code },
      }))
      .then(() => {
        this.setData({ submitting: false, status: '已允许网页登录' });
        wx.showToast({ title: '已允许网页登录', icon: 'success' });
        setTimeout(() => this.back(), 800);
      })
      .catch((err) => {
        this.setData({ submitting: false, status: '' });
        // message 已由 request 统一 toast：
        //   41009 -> 登录会话已过期，请在网页刷新后重试 / 该登录会话已被使用
        //   41020 -> 该微信尚未注册账号，请先在小程序完成注册
        const code = err && err.code;
        if (code === 41009) {
          this.setData({ status: '票号已失效，请在网页刷新后重试' });
        } else if (code === 41020) {
          this.setData({ status: '该微信尚未注册账号' });
        }
      });
  },

  // 扫码即绑定：网页「绑定微信」生成的小程序码 scene=bind:<token>，或扫描 bind:<token> 二维码。
  // 自动 wx.login 拿 code，连同 bind_token 提交，无需用户手动输入任何授权码。
  // 令牌 10 分钟过期、消费即删；过期后网页重新点「绑定微信」刷新即可。
  bindWechat(bindToken) {
    if (this.data.submitting) return;
    this.setData({ submitting: true, status: '正在绑定微信…' });
    auth.bindByToken(bindToken)
      .then(() => {
        this.setData({ submitting: false, status: '微信已绑定' });
        wx.showToast({ title: '微信绑定成功', icon: 'success' });
        // 扫码进入时本页可能是启动页，back() 会按栈情况处理
        setTimeout(() => this.back(), 800);
      })
      .catch((err) => {
        this.setData({ submitting: false, status: '' });
        // message 已由 request 统一 toast：
        //   41009/ErrTokenInvalid -> 绑定已失效，请刷新网页重新获取
        //   41025 -> 该微信已绑定其他账号
        const code = err && err.code;
        if (code === 41009) {
          this.setData({ status: '绑定已失效，请在网页刷新后重试' });
        }
      });
  },

  // 扫描二维码/小程序码：识别网页登录票号或 bind:<token> 自动触发对应流程
  scanCode() {
    if (this.data.submitting) return;
    scan.scanQRCode()
      .then((raw) => {
        const scene = scan.extractScene(raw) || raw;
        if (scene.startsWith('bind:')) {
          this.bindWechat(scene.slice(5));
          return;
        }
        const ticket = scan.extractTicket(raw);
        if (!ticket) {
          wx.showToast({ title: '未能识别网页票号', icon: 'none' });
          return;
        }
        this.setData({ ticket: ticket });
        wx.showToast({ title: '已识别，正在确认', icon: 'success' });
        this.confirm();
      })
      .catch((err) => scan.handleScanFail(err));
  },

  // 扫码进入时本页就是启动页，navigateBack 无栈可退，需回首页
  back() {
    const pages = getCurrentPages();
    if (pages.length > 1) wx.navigateBack();
    else wx.reLaunch({ url: '/pages/index/index' });
  },
});
