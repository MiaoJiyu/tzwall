import http from './http'

export const authApi = {
  register: (p: any) => http.post('/auth/register', p),
  login: (p: any) => http.post('/auth/login', p),
  me: () => http.post('/auth/me'),
  verifyEmail: (token: string) => http.get('/auth/verify-email?token=' + encodeURIComponent(token)),
  resendVerify: (p: any) => http.post('/auth/resend-verify', p),
  forgot: (p: any) => http.post('/auth/forgot', p),
  reset: (p: any) => http.post('/auth/reset', p),
  changePassword: (p: any) => http.post('/auth/change-password', p),
  changeEmail: (p: any) => http.post('/auth/change-email', p),
  logout: () => http.post('/auth/logout'),
  deleteAccount: () => http.post('/auth/delete-account'),
  confirmAgreement: () => http.post('/auth/agreement-confirm'),
}

export const submissionApi = {
  list: (params: any) => http.get('/submissions', { params }),
  get: (id: number) => http.get('/submissions/' + id),
  create: (p: any) => http.post('/submissions', p),
  update: (id: number, p: any) => http.put('/submissions/' + id, p),
  remove: (id: number) => http.delete('/submissions/' + id),
  myList: () => http.get('/my/submissions'),
  tags: () => http.get('/tags'),
  uploadImage: (file: File) => {
    const fd = new FormData()
    fd.append('file', file)
    return http.post('/images', fd)
  },
  imageUrl: (id: number) => '/api/images/' + id,
  uploadVideo: (file: File) => {
    const fd = new FormData()
    fd.append('file', file)
    return http.post('/videos', fd)
  },
  videoUrl: (id: number) => '/api/videos/' + id,
}

export const commentApi = {
  list: (id: number) => http.get('/submissions/' + id + '/comments'),
  create: (id: number, p: any) => http.post('/submissions/' + id + '/comments', p),
  remove: (id: number) => http.delete('/comments/' + id),
}

export const interactionApi = {
  react: (p: any) => http.post('/reactions', p),
}

export const adminApi = {
  dashboard: () => http.get('/admin/dashboard'),
  users: (params: any) => http.get('/admin/users', { params }),
  setRole: (id: number, p: any) => http.post('/admin/users/' + id + '/role', p),
  resendUserVerify: (id: number) => http.post('/admin/users/' + id + '/resend-verify'),
  pendingTags: () => http.get('/admin/tags/pending'),
  approveTag: (id: number, p: any) => http.post('/admin/tags/' + id + '/approve', p),
  mergeTag: (p: any) => http.post('/admin/tags/merge', p),
  allTags: (params?: any) => http.get('/admin/tags', { params }),
  createTag: (p: any) => http.post('/admin/tags', p),
  updateTag: (id: number, p: any) => http.put('/admin/tags/' + id, p),
  deleteTag: (id: number) => http.delete('/admin/tags/' + id),
  activateUser: (id: number) => http.post('/admin/users/' + id + '/activate'),
  pendingSubmissions: (params: any) => http.get('/admin/submissions/pending', { params }),
  allSubmissions: (params: any) => http.get('/admin/submissions/all', { params }),
  reviewSubmission: (id: number, p: any) => http.post('/admin/submissions/' + id + '/review', p),
  restoreSubmission: (id: number) => http.post('/admin/submissions/' + id + '/restore'),
  removeComment: (id: number) => http.post('/admin/comments/' + id + '/remove'),
  restoreComment: (id: number) => http.post('/admin/comments/' + id + '/restore'),
}

export const metaApi = {
  site: () => http.get('/meta/site'),
  agreements: () => http.get('/meta/agreements'),
  agreement: (key: string) => http.get('/meta/agreement?key=' + key),
  routes: () => http.get('/meta/routes'),
}

export const punishmentApi = {
  status: () => http.get('/punishments/status'),
  mine: () => http.get('/punishments/mine'),
  ack: (id: number) => http.post('/punishments/' + id + '/ack'),
  create: (p: any) => http.post('/admin/punishments', p),
  list: (params: any) => http.get('/admin/punishments', { params }),
  revoke: (id: number) => http.post('/admin/punishments/' + id + '/revoke'),
  ipBans: () => http.get('/admin/ip-bans'),
  createIpBan: (p: any) => http.post('/admin/ip-bans', p),
  revokeIpBan: (id: number) => http.post('/admin/ip-bans/' + id + '/revoke'),
}

export const reportApi = {
  create: (p: any) => http.post('/reports', p),
  mine: () => http.get('/reports/mine'),
  list: (params: any) => http.get('/admin/reports', { params }),
  resolve: (id: number, p: any) => http.post('/admin/reports/' + id + '/resolve', p),
  target: (id: number) => http.get('/admin/reports/' + id + '/target'),
}

export const permissionApi = {
  endpoints: () => http.get('/admin/permissions/endpoints'),
  matrix: (params: any) => http.get('/admin/permissions/matrix', { params }),
  save: (p: any) => http.put('/admin/permissions/matrix', p),
}

export const configApi = {
  list: () => http.get('/admin/configs'),
  save: (values: any) => http.put('/admin/configs', { values }),
}

export const agreementApi = {
  list: () => http.get('/admin/agreements'),
  publish: (key: string, p: any) => http.post('/admin/agreements/' + key + '/publish', p),
  rollback: (id: number) => http.post('/admin/agreement-versions/' + id + '/rollback'),
}

export const wordApi = {
  list: (params: any) => http.get('/admin/words', { params }),
  add: (p: any) => http.post('/admin/words', p),
  remove: (id: number) => http.delete('/admin/words/' + id),
}

export const logApi = {
  access: (params: any) => http.get('/admin/logs/access', { params }),
  audit: (params: any) => http.get('/admin/logs/audit', { params }),
}

export const opsApi = {
  overview: () => http.get('/admin/ops/overview'),
}
