import React, { useState, useEffect } from 'react';
import Select from 'react-select';
import { Container, Row, Col, Card } from 'react-bootstrap';
import 'bootstrap/dist/css/bootstrap.min.css';

function App() {
  const [weatherData, setWeatherData] = useState([]);
  const [countries, setCountries] = useState([]);
  const [selectedCountry, setSelectedCountry] = useState(null);
  const [cities, setCities] = useState([]);
  const [selectedCity, setSelectedCity] = useState(null);
  const [socket, setSocket] = useState(null);

  useEffect(() => {
    // Fetch countries from backend
    fetch('http://localhost:8000/countries')
      .then(response => response.json())
      .then(data => {
        const countryOptions = data.map(country => ({
          value: country,
          label: country
        }));
        setCountries(countryOptions);
      })
      .catch(error => console.error('Error fetching countries:', error));
  }, []);

  useEffect(() => {
    if (selectedCountry) {
      // Fetch cities for the selected country from backend
      fetch(`http://localhost:8000/countries/${selectedCountry.value}/cities`)
        .then(response => response.json())
        .then(data => {
          const cityOptions = data.map(city => ({
            value: city,
            label: city.city
          }));
          setCities(cityOptions);
        })
        .catch(error => console.error('Error fetching cities:', error));
    }
  }, [selectedCountry]);

  useEffect(() => {
    if (selectedCity) {
      if (socket) {
        socket.close();
      }
      initiateWS();
    }
  }, [selectedCity]);

  const initiateWS = () => {
    const newSocket = new WebSocket('ws://localhost:8000/ws');

    // Connection opened
    newSocket.addEventListener('open', function (event) {
      // Send initial data
      newSocket.send(JSON.stringify(selectedCity.value._id));
    });

    // Listen for messages
    newSocket.addEventListener('message', function (event) {
      try {
        const jsonData = JSON.parse(event.data);
        setWeatherData(jsonData.sources);
      } catch (error) {
        console.error('Error parsing JSON:', error);
      }
    });

    // Clean up the WebSocket connection when the component unmounts
    newSocket.addEventListener('close', () => {
      setSocket(null);
    });

    setSocket(newSocket);
  };

  return (
    <Container className="mt-5">
      <h1 className="text-center mb-4">Weather Data</h1>
      <Row className="mb-4">
        <Col>
          <Select
            options={countries}
            value={selectedCountry}
            onChange={setSelectedCountry}
            placeholder="Select a country"
            isDisabled={!!selectedCity}
          />
        </Col>
        {selectedCountry && (
          <Col>
            <Select
              options={cities}
              value={selectedCity}
              onChange={setSelectedCity}
              placeholder="Select a city"
              getOptionLabel={(option) => option.label}
              getOptionValue={(option) => JSON.stringify(option.value)}
              isDisabled={!!selectedCity}
            />
          </Col>
        )}
      </Row>
      {selectedCity && weatherData.length > 0 && (
        <Row className="mt-4">
          {weatherData.map((source, index) => (
            <Col key={index} md={4} className="mb-4">
              <Card>
                <Card.Body>
                  <Card.Title>{source.source}</Card.Title>
                  <Card.Text>
                    <strong>Feels Like:</strong> {source.feels_like}°C<br />
                    <strong>Temp Min:</strong> {source.temp_min}°C<br />
                    <strong>Temp Max:</strong> {source.temp_max}°C<br />
                    <strong>Pressure:</strong> {source.pressure} hPa<br />
                    <strong>Humidity:</strong> {source.humidity}%<br />
                    <strong>Temp:</strong> {source.temp}°C
                  </Card.Text>
                </Card.Body>
              </Card>
            </Col>
          ))}
        </Row>
      )}
    </Container>
  );
}

export default App;