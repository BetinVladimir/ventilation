#include <Arduino.h>
#include <WiFi.h>
#include <PubSubClient.h>
#include <ArduinoJson.h>
#include <ESP32Servo.h>
#include "secrets.h"
WiFiClient network;
PubSubClient mqtt(network);
Servo servo;
int position = 0;
unsigned long lastRetry = 0, lastState = 0;
String commandTopic = String("ventilation/") + DEVICE_ID + "/command";
String stateTopic = String("ventilation/") + DEVICE_ID + "/state";
void publishState() {
 JsonDocument doc; doc["position"] = position;
 char payload[128]; serializeJson(doc, payload, sizeof(payload));
 mqtt.publish(stateTopic.c_str(), payload, false);
}
void command(char* topic, byte* payload, unsigned int length) {
 JsonDocument doc;
 if (deserializeJson(doc, payload, length) || !doc["position"].is<int>()) return;
 int target = doc["position"].as<int>();
 if (target < 0 || target > 100) return;
 position = target; servo.write(map(position, 0, 100, 0, 180)); publishState();
}
void setup() {
 Serial.begin(115200);servo.attach(SERVO_PIN);servo.write(0);
 WiFi.mode(WIFI_STA);WiFi.setAutoReconnect(true);WiFi.begin(WIFI_SSID, WIFI_PASSWORD);
 mqtt.setServer(MQTT_HOST, MQTT_PORT);mqtt.setCallback(command);mqtt.setSocketTimeout(2);
}
void loop() {
 unsigned long now = millis();
 if (WiFi.status() == WL_CONNECTED && !mqtt.connected() && now-lastRetry >= 5000) {
  lastRetry = now;
  if (mqtt.connect(DEVICE_ID)) mqtt.subscribe(commandTopic.c_str(), 1);
 }
 mqtt.loop();
 if (mqtt.connected() && now-lastState >= 10000) {lastState = now;publishState();}
 delay(5);
}
