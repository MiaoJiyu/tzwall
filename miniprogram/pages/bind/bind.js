const auth = require('../../utils/auth');
const scan = require('../../utils/scan');

Page({
  data: {
    bindCode: '',
    submitting: false,
  },

  onLoad() {
    // 已登录且微信已绑定：无需重复绑定
    const u = auth.getUser();
    if (u && u.wechat_bound) {
      wx.showToast({ title: '当前微信已绑定', icon: 'none' });
      setTimeout(() => wx.navigateBack(), 800);
    }
  },

  onInput(e) {
    this.setData({ bindCode: e.detail.value });
  },

  // 扫描网页上的绑定二维码/小程序码：识别后自动填入并触发绑定
  scanCode() {
    if (this.data.submitting) return;
    scan.scanQRCode()
      .then((raw) => {
        // 兼容「扫码即绑」令牌：bind:<token> 或 小程序码 scene=bind:<token>
        const scene = scan.extractScene(raw) || raw;
        if (scene.startsWith('bind:')) {
          this.bindByToken(scene.slice(5));
          return;
        }
        const code = scan.extractBindCode(raw);
        if (!code) {
          wx.showToast({ title: '未能识别绑定码', icon: 'none' });
          return;
        }
        this.setData({ bindCode: code });
        wx.showToast({ title: '已识别，正在绑定', icon: 'success' });
        this.submit();
      })
      .catch((err) => scan.handleScanFail(err));
  },

  // 扫码即绑：网页「绑定微信」令牌（bind:<token>）
  bindByToken(bindToken) {
    this.setData({ submitting: true });
    auth.bindByToken(bindToken)
      .then(() => {
        wx.showToast({ title: '微信绑定成功' });
        setTimeout(() => wx.reLaunch({ url: '/pages/login/login' }), 600);
      })
      .catch(() => this.setData({ submitting: false }));
  },

  submit() {
    if (this.data.submitting) return;
    const bindCode = (this.data.bindCode || '').trim().toUpperCase();
    if (!bindCode) {
      wx.showToast({ title: '请输入绑定码', icon: 'none' });
      return;
    }

    this.setData({ submitting: true });
    auth.bindByCode(bindCode)
      .then(() => {
        wx.showToast({ title: '绑定成功' });
        // 绑定后回登录页：此时该微信已绑定账号，wx.login 会直接拿到 token
        setTimeout(() => wx.reLaunch({ url: '/pages/login/login' }), 600);
      })
      .catch((err) => {
        this.setData({ submitting: false });
        const code = err && err.code;
        if (code === 41009) {
          // 绑定码无效或已过期（message 已由 request toast）
        }
        // 41025 该微信已绑定其他账号：message 已透传
      });
  },
});
