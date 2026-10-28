// 登录流程封装：wx.login -> 后端 -> 存 token/user/登录记录。
// 后端契约（新版）：
//   POST /api/auth/wechat/login            {code} -> {token,user,is_new:false} 或 {need_register:true,ticket}
//   POST /api/auth/wechat/register         {ticket,username,password} -> {token,user,is_new:true}
//   POST /api/auth/wechat/bind-code/confirm {bind_code,code} -> 成功 code=0
// 小程序端不处理任何敏感凭证；openid 只能由后端 jscode2session 获取。
const { request, setUnauthorizedHandler } = require('./request');

let inFlight = null; // 并发去重：同一时刻只发一次 wx.login

const RECORD_KEY = 'login_records'; // 本机登录记录
const RECORD_MAX = 20;
const NEED = 'need_register';

function getToken() { return wx.getStorageSync('token'); }
function getUser() { return wx.getStorageSync('user'); }

function setSession(token, user) {
  if (token) wx.setStorageSync('token', token);
  if (user) wx.setStorageSync('user', user);
}

function clearSession() {
  wx.removeStorageSync('token');
  wx.removeStorageSync('user');
}

function formatTime(d) {
  const p = (n) => (n < 10 ? '0' + n : '' + n);
  return d.getFullYear() + '-' + p(d.getMonth() + 1) + '-' + p(d.getDate()) +
    ' ' + p(d.getHours()) + ':' + p(d.getMinutes()) + ':' + p(d.getSeconds());
}

// 记录一次登录，用于「个人登录信息记录」
function recordLogin(user, isNew) {
  if (!user) return;
  let list = [];
  try { list = wx.getStorageSync(RECORD_KEY) || []; } catch (e) { list = []; }
  const now = new Date();
  list.unshift({
    id: now.getTime(),
    time: formatTime(now),
    uid: user.id,
    username: user.username || '',
    nickname: user.nickname || '',
    isNew: !!isNew,
  });
  if (list.length > RECORD_MAX) list = list.slice(0, RECORD_MAX);
  try { wx.setStorageSync(RECORD_KEY, list); } catch (e) { /* 忽略 */ }
}

function getLoginRecords() {
  try { return wx.getStorageSync(RECORD_KEY) || []; } catch (e) { return []; }
}

function clearLoginRecords() {
  try { wx.removeStorageSync(RECORD_KEY); } catch (e) { /* 忽略 */ }
}

// wx.login 拿一次性 code
function wxLoginCode() {
  return new Promise((resolve, reject) => {
    wx.login({
      success(res) {
        if (res.code) resolve(res.code);
        else reject(new Error('获取登录凭证失败'));
      },
      fail() {
        reject(new Error('微信登录调用失败'));
      },
    });
  });
}

// 登录：已绑定账号直接返回 token；未绑定返回 need_register + 一次性票据（10 分钟）
function login() {
  if (inFlight) return inFlight;
  const p = wxLoginCode()
    .then((code) => request({ url: '/api/auth/wechat/login', method: 'POST', data: { code: code } }))
    .then((data) => {
      if (data && data.need_register) {
        return { status: NEED, ticket: data.ticket || '' };
      }
      setSession(data.token, data.user);
      recordLogin(data.user, false);
      return { status: 'ok', user: data.user, isNew: !!data.is_new };
    });
  inFlight = p;
  p.then(() => { inFlight = null; }, () => { inFlight = null; });
  return p;
}

// 首次注册：凭票据设置用户名+密码，成功后该微信自动绑定到新账号
function register(ticket, username, password) {
  return request({
    url: '/api/auth/wechat/register',
    method: 'POST',
    data: { ticket: ticket, username: username, password: password },
  }).then((data) => {
    setSession(data.token, data.user);
    recordLogin(data.user, true);
    return { status: 'ok', user: data.user, isNew: true };
  });
}

// 绑定微信：网页账号中心生成 8 位绑定码，小程序用 wx.login 的 code 提交
function bindByCode(bindCode) {
  return wxLoginCode()
    .then((code) => request({
      url: '/api/auth/wechat/bind-code/confirm',
      method: 'POST',
      data: { bind_code: bindCode, code: code },
    }))
    .then(() => ({ status: 'ok' }));
}

// 扫码即绑：网页「绑定微信」生成的一次性令牌，小程序用 wx.login 的 code 提交。
// 与 bindByCode 对称：一个走手动授权码、一个走扫码自动令牌，均一次性有效（消费即删）。
function bindByToken(bindToken) {
  return wxLoginCode()
    .then((code) => request({
      url: '/api/auth/wechat/bind/confirm',
      method: 'POST',
      data: { bind_token: bindToken, code: code },
    }))
    .then(() => ({ status: 'ok' }));
}

// 校验/取当前用户。401 时会触发 request 层的自动重新登录（适用于用户已主动操作的场景）。
function me() {
  return request({ url: '/api/auth/me', method: 'POST' })
    .then((data) => {
      setSession(getToken(), data);
      return data;
    });
}

// 仅校验本地 token 是否有效，不触发自动重新登录。
// 用于进入页面时的静默会话恢复：token 失效就清掉并展示"未登录"，
// 是否重新登录交给用户主动点击，避免未体验功能即被要求授权。
function checkSession() {
  return request({
    url: '/api/auth/me',
    method: 'POST',
    noRetry: true,
    silent: true,
  }).then((data) => {
    setSession(getToken(), data);
    return data;
  });
}

// 启动登录：本地有 token 先校验，失效则重新登录；本地无 token 直接登录。
// 仅用于用户主动触发登录的场景，不要在页面 onLoad 自动调用。
function ensureLogin() {
  if (getToken()) {
    return me()
      .then((u) => ({ status: 'ok', user: u }))
      .catch(() => login());
  }
  return login();
}

function logout() {
  clearSession();
}

// 把“重新登录”注入到 request 的 401 处理链
setUnauthorizedHandler(login);

module.exports = {
  getToken,
  getUser,
  setSession,
  clearSession,
  wxLoginCode,
  login,
  register,
  bindByCode,
  bindByToken,
  me,
  checkSession,
  ensureLogin,
  logout,
  getLoginRecords,
  clearLoginRecords,
  NEED,
};
