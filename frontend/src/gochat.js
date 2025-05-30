class GoChatClient {
  static MESSAGES_PAGE_LIMIT = 10

  constructor(host) {
    this.host = host;
    this.baseUrl = "http://" + this.host;
  }

  async _request(method, endpoint, data, params = {}) {
    const url = new URL(this.baseUrl + endpoint);

    Object.keys(params).forEach(key => {
      url.searchParams.append(key, params[key]);
    })

    const options = {
      method: method,
      headers: {
        'Content-Type': 'application/json',
        'Accept': 'application/json',
      },
      timeout: 5000
    }

    if (data && ['POST', 'PUT', 'PATCH'].includes(method)) {
      options.body = JSON.stringify(data);
    }

    try {
      const response = await fetch(url, options);

      if (!response.ok) {
        const errorPayload = await response.json();
        throw new Error(errorPayload.message);
      }
      
      if (response.status === 204 || response.headers.get('Content-Length') === '0') {
        return Promise.resolve(null); // Return a resolved promise for no content responses
      }

      return response.json();
    } catch (err) {
      throw new Error(err.message || "An error occurred");
    }
  }

  async listSubscriptions() {
    return this._request('GET', '/api/subscriptions');
  }

  async getRoom(roomId) {
    return this._request('GET', '/api/rooms', null, { id: roomId });
  }

  async subscribeRoom(roomId) {
    return this._request('POST', '/api/subscriptions', null, { room_id: roomId });
  }

  async unsubscribeRoom(roomId) {
    return this._request('DELETE', '/api/subscriptions', null, { room_id: roomId });
  }

  async createRoom(name, description) {
    return this._request('POST', '/api/rooms', { name: name, description: description });
  }

  async deleteRoom(roomId) {
    return this._request('DELETE', '/api/rooms', null, { id: roomId });
  }

  async getMessages(roomId, before = 0) {
    const params = {
      room_id: roomId,
      limit: GoChatClient.MESSAGES_PAGE_LIMIT,
    }

    if (before > 0) {
      params.before = before;
    }

    return this._request('GET', '/api/messages', null, params);
  }

  async getAccount() {
    return this._request('GET', '/api/account');
  }

  async updateAccount(username, password) {
    return this._request('PUT', '/api/account', { username: username, password: password });
  }

  async logout() {
    return this._request('GET', '/api/auth/logout');
  }

  async login(email, password) {
    return this._request('POST', '/api/auth/login', { email: email, password: password });
  }

  async session() {
    return this._request('GET', '/api/auth/session')
  }

  async register(email, username, password) {
    return this._request('POST', '/api/auth/register', { email: email, username: username, password: password });
  }
}

const goChatClient = new GoChatClient(document.location.host);
// const goChatWSClient = new GoChatWSClient("ws://" + document.location.host + "/ws");

export default goChatClient;