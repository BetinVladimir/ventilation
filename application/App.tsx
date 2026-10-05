import React, { useEffect, useState } from 'react';
import { SafeAreaView, ScrollView, Text, TextInput, Button, View, StyleSheet } from 'react-native';
import { StatusBar } from 'expo-status-bar';
const API = process.env.EXPO_PUBLIC_API_URL ?? 'http://localhost:8080';
type Device = { id: string; name: string };
export default function App() {
 const [devices, setDevices] = useState<Device[]>([]);
 const [id, setId] = useState('damper-001');
 const [name, setName] = useState('Заслонка');
 const [position, setPosition] = useState('50');
 const [message, setMessage] = useState('');
 const [busy, setBusy] = useState(false);
 async function request(path: string, body?: unknown) {
  const response = await fetch(API + path, { method: body ? 'POST' : 'GET', headers: { 'Content-Type': 'application/json' }, body: body ? JSON.stringify(body) : undefined });
  const data = await response.json(); if (!response.ok) throw new Error(data.error ?? 'Ошибка запроса'); return data;
 }
 async function run(action: () => Promise<void>) { setBusy(true); try { await action(); } catch (e) { setMessage(e instanceof Error ? e.message : 'Ошибка сети'); } finally { setBusy(false); } }
 async function refresh() { setDevices(await request('/api/devices')); }
 useEffect(() => { void run(refresh); }, []);
 return <SafeAreaView style={styles.screen}><StatusBar style="light" /><ScrollView contentContainerStyle={styles.content}>
  <Text style={styles.title}>Умная вентиляция</Text><Text style={styles.text}>{API}</Text>
  <Button title="Обновить" disabled={busy} onPress={() => void run(refresh)} />
  <TextInput style={styles.input} value={id} onChangeText={setId} placeholder="ID устройства" autoCapitalize="none" />
  <TextInput style={styles.input} value={name} onChangeText={setName} placeholder="Название" />
  <Button title="Добавить устройство" disabled={busy} onPress={() => void run(async () => { await request('/api/devices', {id, name}); await refresh(); setMessage('Устройство добавлено'); })} />
  <TextInput style={styles.input} value={position} onChangeText={setPosition} keyboardType="numeric" placeholder="Позиция 0–100%" />
  {devices.map(d => <View key={d.id} style={styles.card}><Text style={styles.text}>{d.name} · {d.id}</Text>
   <Button title="Установить позицию" disabled={busy} onPress={() => void run(async () => {
    const value = Number(position); if (position.trim() === '' || !Number.isInteger(value) || value < 0 || value > 100) throw new Error('Введите целое число 0–100');
    await request(`/api/devices/${d.id}/position`, {position: value}); setMessage('Команда отправлена брокеру');
   })} />
   <Button title="Получить состояние" disabled={busy} onPress={() => void run(async () => { const state = await request(`/api/telemetry/${d.id}`); setMessage(`${d.name}: ${state.position}% · ${state.received_at}`); })} />
  </View>)}<Text style={styles.text}>{message}</Text>
 </ScrollView></SafeAreaView>;
}
const styles = StyleSheet.create({screen:{flex:1,backgroundColor:'#112229'},content:{padding:24,gap:16},title:{fontSize:28,fontWeight:'700',color:'white'},text:{color:'#ddedf0'},input:{backgroundColor:'white',padding:12,borderRadius:8},card:{padding:16,backgroundColor:'#203b45',borderRadius:12,gap:8}});
