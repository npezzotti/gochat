var loginForm = document.getElementById("login-form")
if (loginForm != null) {
  loginForm.onsubmit = login
}

var logoutBtn = document.getElementById("logout-btn")
if (logoutBtn != null) {
  logoutBtn.onclick = logout
}

async function login(e) {
  e.preventDefault()

  var email = document.getElementById("email").value;
  var password = document.getElementById("password").value;
  var errorMessage = document.getElementById("error-message");
  errorMessage.textContent = '';

  try {
    const response = await fetch("http://" + document.location.host + "/login", {
      method: 'POST',
      headers: { 'Content-type': 'application/json' },
      body: JSON.stringify({ email, password })
    })

    const res = await response.json()
    if (response.status !== 200) {
      throw new Error(res.error || "Login failed")
    }

    if (response.ok) {
      localStorage.setItem("username", res.username)
      window.location.replace('http://localhost:8000/');
    }
  } catch (error) {
    errorMessage.textContent = error.message;
  }
}
