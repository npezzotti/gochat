loginButton = document.getElementById("login-form").onsubmit = login

async function login(e) {
  e.preventDefault()

  var email = document.getElementById("email").value;
  var password = document.getElementById("password").value;
  var errorMessage = document.getElementById("error-message");
  errorMessage.textContent = '';

  try {
    const response = await fetch("http://localhost:8000/login", {
      method: 'POST',
      headers: { 'Content-type': 'application/json' },
      body: JSON.stringify({ email, password })
    })

    const responseTxt = await response.text()
    if (response.status !== 200) {
      throw new Error(responseTxt || "Login failed")
    }

    if (response.redirected) {
      window.location.replace(response.url);
    }
  } catch (error) {
    errorMessage.textContent = error.message;
  }
}
