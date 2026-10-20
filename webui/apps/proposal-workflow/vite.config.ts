export default { server: { host: '127.0.0.1', port: 9800, strictPort: true, proxy: { '/api': 'http://127.0.0.1:8800' } } }
