const auth = require('../../utils/auth');
const agreement = require('../../utils/agreement');
const { request } = require('../../utils/request');
const { absUrl } = require('../../config/index');

// 首页定位：免登录即可浏览的公开内容页。
// 用户先浏览公告与投稿，体验功能服务后，再自行决定是否登录。
// 因此本页不发起任何登录授权，登录入口放在页面底部且为次要按钮。
Page({
  data: {
    announcements: [],
    items: [],
    loading: true,
    errorMsg: '',
    user: null,
    expanded: {}, // 展开的投稿 id
  },

  onLoad() {
    if (!agreement.hasConsented()) {
      wx.reLaunch({ url: '/pages/agreement/agreement' });
      return;
    }
    this.loadContent();
  },

  onShow() {
    if (!agreement.hasConsented()) {
      wx.reLaunch({ url: '/pages/agreement/agreement' });
      return;
    }
    // 仅同步本地已登录用户，不发起登录
    this.setData({ user: auth.getUser() || null });
  },

  onPullDownRefresh() {
    this.loadContent();
    this.setData({ user: auth.getUser() || null });
    wx.stopPullDownRefresh();
  },

  // 公告与投稿相互独立，任一失败不影响另一个展示
  loadContent() {
    this.setData({ loading: true, errorMsg: '' });
    let pending = 2;
    const done = () => {
      pending -= 1;
      if (pending === 0) this.setData({ loading: false });
    };

    request({ url: '/api/announcements', method: 'GET', silent: true })
      .then((d) => {
        this.setData({ announcements: (d && d.items) || [] });
      })
      .catch(() => { /* 公告加载失败不阻塞 */ })
      .then(done);

    request({
      url: '/api/submissions',
      method: 'GET',
      data: { page: 1, page_size: 20, sort: 'new' },
      silent: true,
    })
      .then((d) => {
        const list = ((d && d.items) || []).map(this.normalize);
        this.setData({ items: list, errorMsg: '' });
      })
      .catch((err) => {
        this.setData({
          errorMsg: (err && err.message) || '内容加载失败，请下拉刷新重试',
        });
      })
      .then(done);
  },

  // 统一字段并补全图片绝对地址
  normalize(item) {
    const imgs = (item.images || []).map((im) => absUrl(im.url || im));
    return Object.assign({}, item, {
      cover: imgs[0] || '',
      imgs: imgs,
      timeText: fmtTime(item.created_at),
      summary: item.content || '',
      tags: item.tags || [],
      like_count: item.like_count || 0,
      comment_count: item.comment_count || 0,
      view_count: item.view_count || 0,
      // 兼容后端差异：author / author_title 可能是字符串，也可能（旧版接口）
      // 是对象或嵌套对象，这里统一规整为字符串，避免渲染成 [object Object]
      author: toAuthor(item.author),
      authorTitle: toStr(item.author_title),
      expanded: false,
    });
  },

  toggleItem(e) {
    const id = e.currentTarget.dataset.id;
    const items = this.data.items.map((it) =>
      it.id === id ? Object.assign({}, it, { expanded: !it.expanded }) : it
    );
    this.setData({ items: items });
  },

  previewImage(e) {
    const { urls, current } = e.currentTarget.dataset;
    if (!urls || !urls.length) return;
    wx.previewImage({ urls: urls, current: current || urls[0] });
  },

  // 用户主动进入登录页
  goLogin() {
    wx.navigateTo({ url: '/pages/login/login' });
  },

  goProfile() {
    if (!auth.getToken()) {
      wx.showToast({ title: '登录后可查看', icon: 'none' });
      return;
    }
    wx.switchTab({ url: '/pages/profile/profile' });
  },
});

function fmtTime(d) {
  if (!d) return '';
  const t = new Date(d);
  if (isNaN(t.getTime())) return '';
  const p = (n) => (n < 10 ? '0' + n : '' + n);
  return t.getFullYear() + '-' + p(t.getMonth() + 1) + '-' + p(t.getDate()) +
    ' ' + p(t.getHours()) + ':' + p(t.getMinutes());
}

// 把后端返回的 author 规整成字符串。
// 主流后端返回字符串（如 "Limpide"）；旧版接口可能直接返回用户对象，
// 且该对象里可能再嵌套 title 对象，未做防护时 WXML 会渲染成 [object Object]。
function toAuthor(a) {
  if (a == null) return '匿名用户';
  if (typeof a === 'string') return a;
  if (typeof a === 'object') {
    // 嵌套称号对象（如 {title:{name:'管理员'}}）也要兜底
    const t = a.title;
    return a.name || a.display_name || a.displayName ||
      a.nickname || a.username || a.real_name ||
      (t && (t.name || t.display_name || t.title)) || '匿名用户';
  }
  return String(a);
}

// 把后端返回的展示字段（称号 author_title 等）规整成可读字符串。
// 旧版接口可能把称号序列化为对象（如 {name:'管理员'}），需取出其中的文本。
function toStr(v, fallback) {
  if (v == null) return fallback || '';
  if (typeof v === 'string') return v;
  if (typeof v === 'object') {
    return v.name || v.display_name || v.displayName || v.title || v.value || fallback || '';
  }
  return String(v);
}
