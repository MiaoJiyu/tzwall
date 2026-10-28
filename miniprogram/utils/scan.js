// 扫码封装：调用设备摄像头识别二维码 / 小程序码（微信小程序原生 wx.scanCode）。
//
// 说明：
// - 小程序运行在微信环境内，使用 wx.scanCode 即可调起摄像头，无需 H5 的 getUserMedia；
//   它在微信内置浏览器与主流移动端均原生支持，兼容性由微信保障。
// - scanType 同时包含 'qrCode'（普通二维码）与 'miniProgramCode'（微信小程序码）。
//   对于指向本小程序的小程序码，微信会直接「拉起小程序」并以 scene 进入对应页面
//   （见 app.js onShow / web-login.js onLoad）；扫码按钮则用于识别二维码，以及
//   扫码后由 wx.scanCode 回传 path（形如 pages/web-login/web-login?scene=bind:TOKEN）的场景。
// - 扫描界面为微信原生全屏样式（无法自定义），风格一致性体现在按钮与提示文案上；
//   摄像头权限：首次弹系统授权，用户拒绝后再次调用触发 auth deny，引导前往 wx.openSetting。

function scanQRCode() {
  return new Promise((resolve, reject) => {
    wx.scanCode({
      scanType: ['qrCode', 'miniProgramCode'],
      success(res) {
        // 小程序码常把跳转信息放在 result 或 path；统一取其一并去首尾空白
        const raw = (res.result || res.path || '').trim();
        resolve(raw);
      },
      fail(err) {
        reject(err || new Error('scan canceled'));
      },
    });
  });
}

// 从扫码原文中抽取 scene 值。兼容两种形态：
//   1) 纯 scene：bind:TOKEN / ABCDEF123456
//   2) 小程序码 path：pages/web-login/web-login?scene=bind:TOKEN
function extractScene(raw) {
  if (!raw) return null;
  const m = raw.match(/scene=([^&?#]+)/);
  if (m) {
    try { return decodeURIComponent(m[1]); } catch (e) { return m[1]; }
  }
  return null;
}

// 统一的失败/拒绝处理。返回 true 表示已处理（页面无需再提示）。
// 用户主动取消（cancel）静默忽略，不打扰。
function handleScanFail(err) {
  const msg = (err && err.errMsg) || '';
  if (/cancel/.test(msg)) return true;
  if (/auth deny|permission|authorize/i.test(msg)) {
    wx.showModal({
      title: '需要摄像头权限',
      content: '请在设置中允许使用摄像头，以便扫描二维码',
      confirmText: '去设置',
      cancelText: '暂不',
      success(r) {
        if (r.confirm) wx.openSetting();
      },
    });
    return true;
  }
  wx.showToast({ title: '扫描失败，请重试', icon: 'none' });
  return true;
}

// 从扫描结果中提取绑定码（8 位字母数字）。
// 兼容三种形式：纯码、URL 带 ?code= 参数、混杂文本中的码段、以及小程序码 scene=bind:<token>。
function extractBindCode(raw) {
  if (!raw) return '';
  let s = raw.trim();
  const scene = extractScene(s);
  if (scene) s = scene;
  const p = s.match(/[?&]code=([A-Za-z0-9]{8})/i);
  if (p) return p[1].toUpperCase();
  if (/^[A-Za-z0-9]{8}$/.test(s)) return s.toUpperCase();
  const m = s.match(/[A-Za-z0-9]{8}/);
  return m ? m[0].toUpperCase() : '';
}

// 从扫描结果中提取网页登录票号（12 位大写字母数字）。
function extractTicket(raw) {
  if (!raw) return '';
  let s = raw.trim();
  const scene = extractScene(s);
  if (scene) s = scene;
  const p = s.match(/[?&]ticket=([A-Za-z0-9]{12})/i);
  if (p) return p[1].toUpperCase();
  if (/^[A-Za-z0-9]{12}$/.test(s)) return s.toUpperCase();
  const m = s.match(/[A-Za-z0-9]{12}/);
  return m ? m[0].toUpperCase() : '';
}

module.exports = {
  scanQRCode,
  extractScene,
  handleScanFail,
  extractBindCode,
  extractTicket,
};
