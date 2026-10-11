"""Modal GPU service for EcoSound's optional Qwen nature guide.

Deploy independently with `modal deploy modal/qwen_app.py` from ecosound/.
"""

import subprocess

import modal

MODEL = "Qwen/Qwen3-1.7B"
SERVED_MODEL = "qwen3-1.7b"
HF_CACHE = "/cache/huggingface"
VLLM_CACHE = "/cache/vllm"

hf_volume = modal.Volume.from_name("ecosound-qwen-huggingface-cache", create_if_missing=True)
vllm_volume = modal.Volume.from_name("ecosound-qwen-vllm-cache", create_if_missing=True)

image = (
    modal.Image.from_registry("nvidia/cuda:12.9.0-devel-ubuntu22.04", add_python="3.12")
    .entrypoint([])
    .uv_pip_install("vllm==0.21.0", "huggingface_hub[hf_transfer]==0.36.0")
    .env(
        {
            "HF_HOME": HF_CACHE,
            "HF_HUB_CACHE": f"{HF_CACHE}/hub",
            "HF_XET_HIGH_PERFORMANCE": "1",
            "VLLM_CACHE_ROOT": VLLM_CACHE,
        }
    )
)
app = modal.App("ecosound-qwen-nature-guide", image=image)


@app.function(
    cpu=4,
    memory=16384,
    timeout=900,
    scaledown_window=60,
    volumes={HF_CACHE: hf_volume},
)
def cache_model():
    """Populate the persistent HF cache without holding an L4 during download."""
    from huggingface_hub import snapshot_download

    snapshot_download(repo_id=MODEL, cache_dir=HF_CACHE)
    hf_volume.commit()


@app.function(
    gpu="L4",
    cpu=4,
    memory=24576,
    timeout=900,
    scaledown_window=60,
    volumes={HF_CACHE: hf_volume, VLLM_CACHE: vllm_volume},
)
@modal.web_server(port=8000, startup_timeout=900, requires_proxy_auth=True)
def serve_qwen():
    subprocess.Popen(
        [
            "vllm",
            "serve",
            MODEL,
            "--host",
            "0.0.0.0",
            "--port",
            "8000",
            "--served-model-name",
            SERVED_MODEL,
            "--dtype",
            "bfloat16",
            "--max-model-len",
            "4096",
            "--max-num-seqs",
            "8",
            "--gpu-memory-utilization",
            "0.8",
            "--disable-log-requests",
        ],
        stdout=None,
        stderr=None,
    )
