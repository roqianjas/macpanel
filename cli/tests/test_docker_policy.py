"""Kebijakan keamanan Compose: yang tidak boleh lolos ke `docker compose up`."""

import pytest
from labcli import docker


@pytest.fixture(autouse=True)
def www(tmp_path, monkeypatch):
    w = tmp_path / "wwwroot"
    (w / "app").mkdir(parents=True)
    monkeypatch.setattr(docker, "WWW", w.resolve())
    return w.resolve()


def cfg(**svc):
    return {"services": {"web": svc}}


def errs(c):
    return docker.policy_errors(c)


def test_plain_service_is_allowed():
    assert errs(cfg(image="nginx")) == []


@pytest.mark.parametrize(
    "svc, kata",
    [
        (dict(privileged=True), "privileged"),
        (dict(network_mode="host"), "network_mode"),
        (dict(pid="host"), "pid"),
        (dict(ipc="host"), "ipc"),
        (dict(cap_add=["SYS_ADMIN"]), "cap_add"),
        (dict(devices=["/dev/fuse:/dev/fuse"]), "devices"),
        (dict(security_opt=["seccomp=unconfined"]), "unconfined"),
    ],
)
def test_dangerous_options_are_rejected(svc, kata):
    e = errs(cfg(image="x", **svc))
    assert e and kata in e[0]


def test_ports_must_bind_loopback():
    assert errs(cfg(image="x", ports=[{"published": "8080", "target": 80, "host_ip": "127.0.0.1"}])) == []
    assert errs(cfg(image="x", ports=[{"published": "8080", "target": 80}]))
    assert errs(cfg(image="x", ports=[{"published": "8080", "target": 80, "host_ip": "0.0.0.0"}]))


def test_bind_mounts_only_inside_wwwroot(www):
    inside = {"type": "bind", "source": str(www / "app")}
    assert errs(cfg(image="x", volumes=[inside])) == []
    assert errs(cfg(image="x", volumes=[{"type": "bind", "source": "/etc"}]))
    assert errs(cfg(image="x", volumes=[{"type": "bind", "source": "/var/run/docker.sock"}]))
    assert errs(cfg(image="x", volumes=[{"type": "bind", "source": str(www / ".." / ".ssh")}]))  # lolos lewat ..


def test_named_volume_with_bind_driver_opts_is_rejected(www):
    c = cfg(image="x")
    c["volumes"] = {"v": {"driver": "local", "driver_opts": {"type": "none", "o": "bind", "device": "/Users"}}}
    assert errs(c)
    c["volumes"] = {"v": {"driver": "local", "driver_opts": {"type": "none", "o": "bind", "device": str(www / "app")}}}
    assert errs(c) == []
    c["volumes"] = {"v": {}}
    assert errs(c) == []
    c["volumes"] = {"v": {"driver": "nfs"}}
    assert errs(c)


def test_build_context_must_be_inside_wwwroot(www):
    assert errs(cfg(build={"context": str(www / "app")})) == []
    assert errs(cfg(build={"context": "/Users"}))
    assert errs(cfg(build="/tmp"))


def test_sysctls_rejected():
    assert errs(cfg(image="x", sysctls={"net.ipv4.ip_forward": "1"}))


def test_errors_name_the_service():
    c = {"services": {"api": {"privileged": True}, "db": {"image": "x"}}}
    e = errs(c)
    assert len(e) == 1 and e[0].startswith("api:")
