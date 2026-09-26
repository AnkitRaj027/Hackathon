// Vault Three.js Programmatic Materials
// Programmatic material architecture calibrated for industrial hardware realism.

import * as THREE from 'three';
import { StateColors, BaseColors } from './tokens';

export class MaterialRegistry {
  // Chassis materials — industrial server rack aesthetics
  public static readonly chassisBase = new THREE.MeshStandardMaterial({
    color: new THREE.Color('#12151e'),
    metalness: 0.75,
    roughness: 0.4,
  });

  public static readonly chassisBezel = new THREE.MeshStandardMaterial({
    color: new THREE.Color('#0a0c12'),
    metalness: 0.85,
    roughness: 0.3,
  });

  public static readonly baySlot = new THREE.MeshStandardMaterial({
    color: new THREE.Color('#08090d'),
    metalness: 0.3,
    roughness: 0.8,
  });

  public static readonly ringMaterial = new THREE.MeshBasicMaterial({
    color: new THREE.Color(BaseColors.ringTrack),
    transparent: true,
    opacity: 0.4,
  });

  public static readonly ringMarkerMaterial = new THREE.MeshBasicMaterial({
    color: new THREE.Color(BaseColors.borderActive),
    transparent: true,
    opacity: 0.35,
  });

  public static readonly linkMaterial = new THREE.LineBasicMaterial({
    color: new THREE.Color(BaseColors.linkCurve),
    transparent: true,
    opacity: 0.3,
  });

  public static readonly linkSelectedMaterial = new THREE.LineBasicMaterial({
    color: new THREE.Color(BaseColors.linkHighlight),
    transparent: true,
    opacity: 0.85,
  });

  // State LED / Indicator materials — restrained, precise
  private static ledMaterials: Record<string, THREE.MeshStandardMaterial> = {};

  public static getLedMaterial(state: string, isSelected: boolean = false): THREE.MeshStandardMaterial {
    const key = `${state}-${isSelected ? 'sel' : 'norm'}`;
    if (!this.ledMaterials[key]) {
      let colorHex: string = StateColors.HEALTHY;
      let emissiveIntensity = 0.5;

      switch (state) {
        case 'DEAD':
          colorHex = StateColors.DEAD;
          emissiveIntensity = 0.15;
          break;
        case 'SUSPECT':
          colorHex = StateColors.SUSPECT;
          emissiveIntensity = 0.65;
          break;
        case 'DEGRADED':
          colorHex = StateColors.DEGRADED;
          emissiveIntensity = 0.7;
          break;
        case 'REPAIRING':
          colorHex = StateColors.REPAIRING;
          emissiveIntensity = 0.85;
          break;
        case 'CORRUPTED':
          colorHex = StateColors.CORRUPTED;
          emissiveIntensity = 0.95;
          break;
        default:
          colorHex = StateColors.HEALTHY;
          emissiveIntensity = 0.5;
          break;
      }

      if (isSelected) {
        emissiveIntensity += 0.35;
      }

      const color = new THREE.Color(colorHex);
      this.ledMaterials[key] = new THREE.MeshStandardMaterial({
        color: color,
        emissive: color,
        emissiveIntensity: emissiveIntensity,
        roughness: 0.25,
      });
    }
    return this.ledMaterials[key];
  }

  // Chunk status materials
  private static chunkMaterials: Record<string, THREE.MeshStandardMaterial> = {};

  public static getChunkMaterial(isParity: boolean, isCorrupt: boolean = false, isSelected: boolean = false): THREE.MeshStandardMaterial {
    const key = `${isParity ? 'parity' : 'data'}-${isCorrupt ? 'corrupt' : 'clean'}-${isSelected ? 'sel' : 'norm'}`;
    if (!this.chunkMaterials[key]) {
      let color = new THREE.Color(BaseColors.accent);
      if (isCorrupt) {
        color = new THREE.Color(StateColors.CORRUPTED);
      } else if (isParity) {
        color = new THREE.Color('#a78bfa');
      }

      this.chunkMaterials[key] = new THREE.MeshStandardMaterial({
        color: color,
        emissive: color,
        emissiveIntensity: isSelected ? 0.75 : 0.25,
        metalness: 0.35,
        roughness: 0.45,
      });
    }
    return this.chunkMaterials[key];
  }
}
