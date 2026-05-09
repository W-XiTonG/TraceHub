import axios from 'axios'

export const http = axios.create({
  baseURL: '',
  timeout: 120000,
})

http.interceptors.request.use((config) => {
  const token = localStorage.getItem('tracehub_token')
  if (token) {
    config.headers.Authorization = `Bearer ${token}`
  }
  return config
})

export type Envelope<T> = { code: number; message?: string; data: T }
