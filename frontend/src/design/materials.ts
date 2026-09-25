// Vault Three.js Programmatic Materials
// Section 54 & 89: Programmatic material architecture driven by real state.

import * as THREE from 'three';
import { StateColors, BaseColors } from './tokens';

export class MaterialRegistry {
  // Chassis materials
  public static readonly chassisBase = new THREE.MeshStandardMaterial({
    color: new THREE.Color('#141c2b'),
    metalness: 0.85,
    roughness: 0.35,
  });

  public static readonly chassisBezel = new THREE.MeshStandardMaterial({
    color: new THREE.Color('#0a0f18'),
    metalness: 0.9,
    roughness: 0.25,
  });

  public static readonly baySlot = new THREE.MeshStandardMaterial({
    color: new THREE.Color('#080c14'),
    metalness: 0.5,
    roughness: 0.7,
  });

  public static readonly ringMaterial = new THREE.MeshBasicMaterial({
    color: new THREE.Color(BaseColors.ringTrack),
    transparent: true,
    opacity: 0.45,
  });

  public static readonly ringMarkerMaterial = new THREE.MeshBasicMaterial({
    color: new THREE.Color(BaseColors.textMuted),
    transparent: true,
    opacity: 0.3,
  });

  public static readonly linkMaterial = new THREE.LineBasicMaterial({
    color: new THREE.Color(BaseColors.linkCurve),
    transparent: true,
    opacity: 0.35,
  });

  public static readonly linkSelectedMaterial = new THREE.LineBasicMaterial({
    color: new THREE.Color(BaseColors.linkHighlight),
    transparent: true,
    opacity: 0.95,
  });

  // State LED / Indicator materials
  private static ledMaterials: Record<string, THREE.MeshStandardMaterial> = {};

  public static getLedMaterial(state: string, isSelected: boolean = false): THREE.MeshStandardMaterial {
    const key = `${state}-${isSelected ? 'sel' : 'norm'}`;
    if (!this.ledMaterials[key]) {
      let colorHex: string = StateColors.HEALTHY;
      let emissiveIntensity = 0.6;

      switch (state) {
        case 'DEAD':
          colorHex = StateColors.DEAD;
          emissiveIntensity = 0.15;
          break;
        case 'SUSPECT':
          colorHex = StateColors.SUSPECT;
          emissiveIntensity = 0.8;
          break;
        case 'DEGRADED':
          colorHex = StateColors.DEGRADED;
          emissiveIntensity = 0.85;
          break;
        case 'REPAIRING':
          colorHex = StateColors.REPAIRING;
          emissiveIntensity = 1.0;
          break;
        case 'CORRUPTED':
          colorHex = StateColors.CORRUPTED;
          emissiveIntensity = 1.2;
          break;
        default:
          colorHex = StateColors.HEALTHY;
          emissiveIntensity = 0.6;
          break;
      }

      if (isSelected) {
        emissiveIntensity += 0.5;
      }

      const color = new THREE.Color(colorHex);
      this.ledMaterials[key] = new THREE.MeshStandardMaterial({
        color: color,
        emissive: color,
        emissiveIntensity: emissiveIntensity,
        roughness: 0.2,
      });
    }
    return this.ledMaterials[key];
  }

  // Chunk status materials
  private static chunkMaterials: Record<string, THREE.MeshStandardMaterial> = {};

  public static getChunkMaterial(isParity: boolean, isCorrupt: boolean = false, isSelected: boolean = false): THREE.MeshStandardMaterial {
    const key = `${isParity ? 'parity' : 'data'}-${isCorrupt ? 'corrupt' : 'clean'}-${isSelected ? 'sel' : 'norm'}`;
    if (!this.chunkMaterials[key]) {
      let color = new THREE.Color('#38bdf8');
      if (isCorrupt) {
        color = new THREE.Color(StateColors.CORRUPTED);
      } else if (isParity) {
        color = new THREE.Color('#a855f7'); // distinct parity chunk indicator
      }

      this.chunkMaterials[key] = new THREE.MeshStandardMaterial({
        color: color,
        emissive: color,
        emissiveIntensity: isSelected ? 0.9 : 0.35,
        metalness: 0.4,
        roughness: 0.4,
      });
    }
    return this.chunkMaterials[key];
  }
}
